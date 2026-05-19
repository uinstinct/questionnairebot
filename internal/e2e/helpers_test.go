//go:build integration

// Package e2e holds end-to-end tests that drive the bot against a real Telegram
// test bot. These tests FAIL when TEST_TELEGRAM_BOT_TOKEN and
// TEST_TELEGRAM_CHAT_ID are not set in the environment (or in a .env file at
// the project root). TELEGRAM_BOT_TOKEN / TELEGRAM_CHAT_ID from .env are used
// as fallbacks when the TEST_-prefixed vars are absent.
//
// Run with: go test ./... -tags integration
package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"

	"github.com/aditya-mitra/questionnairebot/internal/bot"
	"github.com/aditya-mitra/questionnairebot/internal/commands"
	"github.com/aditya-mitra/questionnairebot/internal/handler"
	"github.com/aditya-mitra/questionnairebot/internal/loader"
	"github.com/aditya-mitra/questionnairebot/internal/session"
)

// TestMain loads a .env file from the project root (if present) before any
// tests run so that secrets stored there are available as environment variables.
func TestMain(m *testing.M) {
	if path := findDotEnv(); path != "" {
		// Overload so explicit env vars set by CI always win.
		_ = godotenv.Overload(path)
	}
	os.Exit(m.Run())
}

// findDotEnv walks up from the current working directory until it finds a .env
// file or reaches the filesystem root.
func findDotEnv() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(dir, ".env")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// firstEnv returns the value of the first non-empty environment variable from
// the provided list.
func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

// requireTestEnv reads TEST_TELEGRAM_BOT_TOKEN and TEST_TELEGRAM_CHAT_ID (with
// fallback to TELEGRAM_BOT_TOKEN / TELEGRAM_CHAT_ID from .env) and fails the
// calling test if either is absent.
func requireTestEnv(t *testing.T) (string, int64) {
	t.Helper()
	token := firstEnv("TEST_TELEGRAM_BOT_TOKEN", "TELEGRAM_BOT_TOKEN")
	chatRaw := firstEnv("TEST_TELEGRAM_CHAT_ID", "TELEGRAM_CHAT_ID")
	if token == "" || chatRaw == "" {
		t.Fatal("E2E requires TEST_TELEGRAM_BOT_TOKEN and TEST_TELEGRAM_CHAT_ID (or their non-prefixed .env equivalents)")
	}
	chatID, err := strconv.ParseInt(chatRaw, 10, 64)
	require.NoError(t, err, "TEST_TELEGRAM_CHAT_ID must be a signed int64")
	return token, chatID
}

// e2eSender wraps the real bot for actual Telegram sends and simultaneously
// records every outgoing message to a buffered channel. This lets probeClient
// observe bot output without a second getUpdates connection, which would
// 409-conflict with the bot's own long-poll.
type e2eSender struct {
	real *bot.Bot
	out  chan tgbotapi.Message
}

func (s *e2eSender) Send(text string) error {
	s.out <- tgbotapi.Message{Text: text, From: &tgbotapi.User{IsBot: true}}
	return s.real.Send(text)
}

func (s *e2eSender) SendMarkdown(text string) error {
	s.out <- tgbotapi.Message{Text: text, From: &tgbotapi.User{IsBot: true}}
	return s.real.SendMarkdown(text)
}

func (s *e2eSender) SendPicker(text string, options []bot.PickerOption) error {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(options))
	for _, opt := range options {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(opt.Label, opt.CallbackData),
		))
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	s.out <- tgbotapi.Message{Text: text, From: &tgbotapi.User{IsBot: true}, ReplyMarkup: &kb}
	return s.real.SendPicker(text, options)
}

func (s *e2eSender) AckCallback(callbackID string) error {
	return s.real.AckCallback(callbackID)
}

func (s *e2eSender) logUserAction(text string) {
	_ = s.real.Send("👤 " + text)
}

// botRig is the in-process bot under test: same wiring as cmd/bot/main.go.
type botRig struct {
	t      *testing.T
	chatID int64
	disp   *handler.Dispatcher
	sender *e2eSender
	bus    *commands.CronBus
	out    chan tgbotapi.Message
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// newBotUnderTest builds the same component graph as cmd/bot/main.go against
// dataDir + (token, chatID). Returns the rig and a teardown function the test
// MUST defer.
//
// clock overrides the time source used by the pull/cron/status/list commands and
// flow.Now. Pass nil to use time.Now.
//
// b.Run (the getUpdates long-poll goroutine) is intentionally NOT started.
// User messages are injected in-process via probeClient.send, so there is
// no second-poller 409 Conflict on the shared bot token.
func newBotUnderTest(t *testing.T, dataDir string, token string, chatID int64, clock func() time.Time) (*botRig, func()) {
	t.Helper()
	if clock == nil {
		clock = time.Now
	}
	qs, err := loader.Load(dataDir)
	require.NoError(t, err, "loader.Load")

	sessions := session.NewManager(dataDir)
	flow := handler.New(nil, sessions, dataDir, qs)
	flow.Now = clock
	disp := handler.NewDispatcher(flow)

	b, err := bot.New(token, chatID, disp)
	require.NoError(t, err, "bot.New")

	out := make(chan tgbotapi.Message, 32)
	sender := &e2eSender{real: b, out: out}
	flow.Sender = sender

	require.NoError(t, handler.Restore(flow))

	ctx, cancel := context.WithCancel(context.Background())
	bus := commands.NewCronBus(flow, sender, clock)

	pull := commands.NewPull(flow, clock)
	status := commands.NewStatus(dataDir, sessions, flow.Questionnaires, clock)
	list := commands.NewList(flow.Questionnaires, clock)
	disp.Attach(commands.NewAdapter(pull, status, list))

	rig := &botRig{t: t, chatID: chatID, disp: disp, sender: sender, bus: bus, out: out, cancel: cancel}

	rig.wg.Add(1)
	go func() { defer rig.wg.Done(); bus.Run(ctx) }()

	teardown := func() {
		cancel()
		done := make(chan struct{})
		go func() { rig.wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Log("e2e teardown: goroutines did not exit within 5s")
		}
	}
	return rig, teardown
}

// inject dispatches a synthetic Telegram update (slash command or free-text)
// directly into the dispatcher, bypassing Telegram's transport layer entirely.
func (r *botRig) inject(text string) {
	r.sender.logUserAction(text)
	chat := &tgbotapi.Chat{ID: r.chatID}
	var update tgbotapi.Update
	if strings.HasPrefix(text, "/") {
		cmd := strings.SplitN(text, " ", 2)[0]
		update = tgbotapi.Update{Message: &tgbotapi.Message{
			Chat: chat,
			Text: text,
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: len(cmd)},
			},
		}}
	} else {
		update = tgbotapi.Update{Message: &tgbotapi.Message{Chat: chat, Text: text}}
	}
	r.disp.Handle(context.Background(), r.sender, update)
}

// probeClient is the test-side observer. It reads outgoing bot messages from
// the recording channel and injects user messages directly into the dispatcher.
// No getUpdates polling is performed — there is no second-poller conflict.
type probeClient struct {
	rig *botRig
	mu  sync.Mutex
	log []tgbotapi.Message
}

func newProbeClient(t *testing.T, rig *botRig) *probeClient {
	t.Helper()
	return &probeClient{rig: rig}
}

// waitForMessage drains the outgoing channel until predicate returns true or
// timeout elapses. Non-matching messages are buffered in log for diagnostics.
func (p *probeClient) waitForMessage(t *testing.T, timeout time.Duration, predicate func(tgbotapi.Message) bool) tgbotapi.Message {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		select {
		case msg := <-p.rig.out:
			p.mu.Lock()
			p.log = append(p.log, msg)
			p.mu.Unlock()
			if predicate(msg) {
				return msg
			}
		case <-time.After(remaining):
			// deadline reached
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	t.Fatalf("probe.waitForMessage: timeout after %s. Recent bot messages: %+v", timeout, lastN(p.log, 5))
	return tgbotapi.Message{}
}

// send injects a free-text or slash-command message from the simulated user.
func (p *probeClient) send(t *testing.T, text string) {
	t.Helper()
	p.rig.inject(text)
}

// sendCallback injects callback data as a free-text message (best-effort
// approximation — inline-keyboard taps require a user account, not a bot token).
func (p *probeClient) sendCallback(t *testing.T, data string) {
	t.Helper()
	p.rig.inject(data)
}

func lastN[T any](s []T, n int) []T {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
