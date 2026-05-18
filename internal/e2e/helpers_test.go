//go:build integration

// Package e2e holds end-to-end tests that drive the bot against a real Telegram
// test bot. These tests SKIP (not fail) when TEST_TELEGRAM_BOT_TOKEN and
// TEST_TELEGRAM_CHAT_ID are not set in the environment.
//
// Run with: go test ./... -tags integration
package e2e

import (
	"context"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/require"

	"github.com/aditya-mitra/questionnairebot/internal/bot"
	"github.com/aditya-mitra/questionnairebot/internal/commands"
	"github.com/aditya-mitra/questionnairebot/internal/handler"
	"github.com/aditya-mitra/questionnairebot/internal/loader"
	"github.com/aditya-mitra/questionnairebot/internal/session"
)

// requireTestEnv reads TEST_TELEGRAM_BOT_TOKEN and TEST_TELEGRAM_CHAT_ID and
// skips the calling test if either is absent.
func requireTestEnv(t *testing.T) (string, int64) {
	t.Helper()
	token := os.Getenv("TEST_TELEGRAM_BOT_TOKEN")
	chatRaw := os.Getenv("TEST_TELEGRAM_CHAT_ID")
	if token == "" || chatRaw == "" {
		t.Skip("E2E requires TEST_TELEGRAM_BOT_TOKEN and TEST_TELEGRAM_CHAT_ID")
	}
	chatID, err := strconv.ParseInt(chatRaw, 10, 64)
	require.NoError(t, err, "TEST_TELEGRAM_CHAT_ID must be a signed int64")
	return token, chatID
}

// botRig is the in-process bot under test: same wiring as cmd/bot/main.go.
type botRig struct {
	t         *testing.T
	bot       *bot.Bot
	bus       *commands.CronBus
	flow      *handler.QuestionFlow
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// newBotUnderTest builds the same component graph as cmd/bot/main.go against
// dataDir + (token, chatID). Returns the rig and a teardown function the test
// MUST defer.
func newBotUnderTest(t *testing.T, dataDir string, token string, chatID int64) (*botRig, func()) {
	t.Helper()
	qs, err := loader.Load(dataDir)
	require.NoError(t, err, "loader.Load")

	sessions := session.NewManager(dataDir)
	flow := handler.New(nil, sessions, dataDir, qs)
	disp := handler.NewDispatcher(flow)

	b, err := bot.New(token, chatID, disp)
	require.NoError(t, err, "bot.New")
	flow.Sender = b

	require.NoError(t, handler.Restore(flow))

	ctx, cancel := context.WithCancel(context.Background())
	bus := commands.NewCronBus(flow, b, time.Now)

	pull := commands.NewPull(flow, time.Now)
	status := commands.NewStatus(dataDir, sessions, flow.Questionnaires, time.Now)
	list := commands.NewList(flow.Questionnaires, time.Now)
	disp.Attach(commands.NewAdapter(pull, status, list))

	rig := &botRig{t: t, bot: b, bus: bus, flow: flow, cancel: cancel}

	rig.wg.Add(2)
	go func() { defer rig.wg.Done(); bus.Run(ctx) }()
	go func() { defer rig.wg.Done(); b.Run(ctx) }()

	teardown := func() {
		cancel()
		// Give the polling goroutines a beat to exit cleanly.
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

// probeClient is the "test user side" — a second telegram-bot-api client that
// reads what the bot under test sent and writes inputs back via sendMessage.
type probeClient struct {
	api    *tgbotapi.BotAPI
	chatID int64
	offset int
	mu     sync.Mutex
	log    []tgbotapi.Message
}

func newProbeClient(t *testing.T, token string, chatID int64) *probeClient {
	t.Helper()
	api, err := tgbotapi.NewBotAPI(token)
	require.NoError(t, err, "probe NewBotAPI")
	return &probeClient{api: api, chatID: chatID}
}

// waitForMessage polls getUpdates until predicate returns true for a message in
// the configured chat, or timeout elapses. Returns the matching message.
func (p *probeClient) waitForMessage(t *testing.T, timeout time.Duration, predicate func(tgbotapi.Message) bool) tgbotapi.Message {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		u := tgbotapi.NewUpdate(p.offset)
		u.Timeout = 1
		updates, err := p.api.GetUpdates(u)
		if err != nil {
			t.Logf("probe getUpdates: %v", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		for _, up := range updates {
			p.offset = up.UpdateID + 1
			if up.Message == nil {
				continue
			}
			if up.Message.Chat == nil || up.Message.Chat.ID != p.chatID {
				continue
			}
			// Only look at messages FROM the bot (not echoes of our own probe sends).
			if up.Message.From != nil && up.Message.From.IsBot {
				p.mu.Lock()
				p.log = append(p.log, *up.Message)
				p.mu.Unlock()
				if predicate(*up.Message) {
					return *up.Message
				}
			}
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	t.Fatalf("probe.waitForMessage: timeout after %s. Recent bot messages: %+v", timeout, lastN(p.log, 5))
	return tgbotapi.Message{}
}

// send dispatches a free-text reply from the probe (test-user side) to the bot.
func (p *probeClient) send(t *testing.T, text string) {
	t.Helper()
	msg := tgbotapi.NewMessage(p.chatID, text)
	_, err := p.api.Send(msg)
	require.NoError(t, err, "probe.send")
}

// sendCallback simulates tapping an inline-keyboard button by issuing the
// callback data through the Telegram answerCallbackQuery flow. Because the
// probe is itself a bot account, the only viable approximation in pure test
// code is to send the callback data as a plain message and rely on the bot's
// /pull callback handling — but the bot's dispatcher only routes
// CallbackQuery updates for inline keyboards. For end-to-end button-tap
// behavior, run the test interactively or extend the probe to use a user
// account (not implemented here — surfaced as a known limitation).
func (p *probeClient) sendCallback(t *testing.T, data string) {
	t.Helper()
	// Best-effort: send the slug parsed out of the callback data as a free-text
	// message. The bot will treat it as an answer if a session is active; the
	// dual-pending test uses the cron-fire path instead of /pull to validate
	// picker contents.
	p.send(t, data)
}

func lastN[T any](s []T, n int) []T {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
