package handler

import (
	"context"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/aditya-mitra/questionnairebot/internal/bot"
	"github.com/aditya-mitra/questionnairebot/internal/loader"
	"github.com/aditya-mitra/questionnairebot/internal/session"
)

func newFlow(t *testing.T, sender Sender, qs []*loader.Questionnaire) (*QuestionFlow, string) {
	t.Helper()
	tmp := t.TempDir()
	sessions := session.NewManager(tmp)
	flow := New(sender, sessions, tmp, qs)
	flow.Now = func() time.Time { return time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC) }
	return flow, tmp
}

func cmdUpdate(text string) tgbotapi.Update {
	chat := &tgbotapi.Chat{ID: 1}
	msg := &tgbotapi.Message{
		MessageID: 1,
		Chat:      chat,
		Text:      text,
		Entities:  []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: len(strings.Split(text, " ")[0])}},
	}
	return tgbotapi.Update{Message: msg}
}

func freeTextUpdate(text string) tgbotapi.Update {
	chat := &tgbotapi.Chat{ID: 1}
	return tgbotapi.Update{Message: &tgbotapi.Message{MessageID: 1, Chat: chat, Text: text}}
}

func editedTextUpdate(text string, id int) tgbotapi.Update {
	chat := &tgbotapi.Chat{ID: 1}
	return tgbotapi.Update{EditedMessage: &tgbotapi.Message{MessageID: id, Chat: chat, Text: text}}
}

func editedCmdUpdate(text string, id int) tgbotapi.Update {
	chat := &tgbotapi.Chat{ID: 1}
	msg := &tgbotapi.Message{
		MessageID: id,
		Chat:      chat,
		Text:      text,
		Entities:  []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: len(strings.Split(text, " ")[0])}},
	}
	return tgbotapi.Update{EditedMessage: msg}
}

func TestDispatcherSlashHelp(t *testing.T) {
	sender := &recordingSender{}
	flow, _ := newFlow(t, sender, nil)
	d := NewDispatcher(flow)
	d.Handle(context.Background(), sender, cmdUpdate("/help"))
	if len(sender.msgs) != 1 || sender.msgs[0] != HelpText {
		t.Fatalf("msgs = %v", sender.msgs)
	}
}

func TestDispatcherSlashStubs(t *testing.T) {
	for _, cmd := range []string{"pull", "status", "list"} {
		sender := &recordingSender{}
		flow, _ := newFlow(t, sender, nil)
		d := NewDispatcher(flow)
		d.Handle(context.Background(), sender, cmdUpdate("/"+cmd))
		want := "Phase 4 will implement /" + cmd + "."
		if len(sender.msgs) != 1 || sender.msgs[0] != want {
			t.Errorf("/%s msgs = %v, want %q", cmd, sender.msgs, want)
		}
	}
}

func TestDispatcherUnknownCommand(t *testing.T) {
	sender := &recordingSender{}
	flow, _ := newFlow(t, sender, nil)
	d := NewDispatcher(flow)
	d.Handle(context.Background(), sender, cmdUpdate("/somethingelse"))
	if len(sender.msgs) != 1 || sender.msgs[0] != "Unknown command." {
		t.Fatalf("msgs = %v", sender.msgs)
	}
}

func TestDispatcherFreeTextNoSession(t *testing.T) {
	sender := &recordingSender{}
	q := &loader.Questionnaire{Slug: "x", Name: "X", Location: time.UTC, Questions: []loader.Question{{Question: "Q?"}}}
	flow, _ := newFlow(t, sender, []*loader.Questionnaire{q})
	d := NewDispatcher(flow)
	d.Handle(context.Background(), sender, freeTextUpdate("hello"))
	if len(sender.msgs) != 1 || sender.msgs[0] != HelpText {
		t.Fatalf("msgs = %v", sender.msgs)
	}
}

type fakeCommands struct {
	pullCalled    bool
	pullErr       error
	statusText    string
	listText      string
	cbData        string
	cbErr         error
}

func (f *fakeCommands) HandlePull(sender bot.Sender) error {
	f.pullCalled = true
	if f.pullErr != nil {
		return f.pullErr
	}
	return sender.Send("pull ok")
}
func (f *fakeCommands) RenderStatus() string                                { return f.statusText }
func (f *fakeCommands) RenderList() string                                  { return f.listText }
func (f *fakeCommands) HandleStartCallback(sender bot.Sender, data string) error {
	f.cbData = data
	if f.cbErr != nil {
		return f.cbErr
	}
	return sender.Send("cb ok: " + data)
}

func TestDispatcherPullRoutes(t *testing.T) {
	sender := &recordingSender{}
	flow, _ := newFlow(t, sender, nil)
	d := NewDispatcher(flow)
	fakes := &fakeCommands{}
	d.Attach(fakes)
	d.Handle(context.Background(), sender, cmdUpdate("/pull"))
	if !fakes.pullCalled {
		t.Fatalf("HandlePull not called")
	}
	if len(sender.msgs) != 1 || sender.msgs[0] != "pull ok" {
		t.Errorf("msgs = %v", sender.msgs)
	}
}

func TestDispatcherStatusList(t *testing.T) {
	sender := &recordingSender{}
	flow, _ := newFlow(t, sender, nil)
	d := NewDispatcher(flow)
	d.Attach(&fakeCommands{statusText: "📊 Status: ok", listText: "📋 Questionnaires: ok"})
	d.Handle(context.Background(), sender, cmdUpdate("/status"))
	d.Handle(context.Background(), sender, cmdUpdate("/list"))
	if len(sender.msgs) != 2 || !strings.Contains(sender.msgs[0], "📊 Status") || !strings.Contains(sender.msgs[1], "📋 Questionnaires") {
		t.Fatalf("msgs = %v", sender.msgs)
	}
}

func TestDispatcherCallbackStart(t *testing.T) {
	sender := &recordingSender{}
	flow, _ := newFlow(t, sender, nil)
	d := NewDispatcher(flow)
	fakes := &fakeCommands{}
	d.Attach(fakes)
	cb := &tgbotapi.CallbackQuery{ID: "cb1", Data: "start:daily:2026-05-19T09:00:00Z"}
	d.Handle(context.Background(), sender, tgbotapi.Update{CallbackQuery: cb})
	if fakes.cbData != "start:daily:2026-05-19T09:00:00Z" {
		t.Errorf("cbData = %q", fakes.cbData)
	}
	if len(sender.acks) != 1 || sender.acks[0] != "cb1" {
		t.Errorf("acks = %v", sender.acks)
	}
}

func TestDispatcherFreeTextActiveSession(t *testing.T) {
	sender := &recordingSender{}
	q := &loader.Questionnaire{Slug: "x", Name: "X", Location: time.UTC, Questions: []loader.Question{{Question: "Q1?"}}}
	flow, _ := newFlow(t, sender, []*loader.Questionnaire{q})
	d := NewDispatcher(flow)

	now := flow.Now()
	if err := flow.StartQuestionnaire("x", now); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(sender.msgs) != 1 || sender.msgs[0] != "Q1?" {
		t.Fatalf("after Start = %v", sender.msgs)
	}
	d.Handle(context.Background(), sender, freeTextUpdate("my answer"))
	// After the answer, the single-question questionnaire finalises.
	if len(sender.msgs) != 2 || !strings.Contains(sender.msgs[1], "✅ X complete!") {
		t.Fatalf("after answer = %v", sender.msgs)
	}
}

func startWithAnswer(t *testing.T, flow *QuestionFlow, slug string, id int) {
	t.Helper()
	t0 := flow.Now()
	if _, err := flow.Sessions.Start(slug, t0, t0, time.UTC); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := flow.Sessions.RecordAnswer(slug, "Q1?", "A1", id); err != nil {
		t.Fatalf("RecordAnswer: %v", err)
	}
}

func editQ(t *testing.T) []*loader.Questionnaire {
	t.Helper()
	return []*loader.Questionnaire{{
		Slug: "x", Name: "X", Location: time.UTC,
		Questions: []loader.Question{{Question: "Q1?"}, {Question: "Q2?"}},
	}}
}

func TestDispatcherEditRoutesAndUpdates(t *testing.T) {
	sender := &recordingSender{}
	flow, _ := newFlow(t, sender, editQ(t))
	d := NewDispatcher(flow)
	startWithAnswer(t, flow, "x", 42)

	d.Handle(context.Background(), sender, editedTextUpdate("edited", 42))

	if got := flow.Sessions.Get("x"); got.Answers[0].Answer != "edited" {
		t.Fatalf("answer = %q, want edited", got.Answers[0].Answer)
	}
	if len(sender.msgs) == 0 || !strings.Contains(sender.msgs[len(sender.msgs)-1], "Updated") {
		t.Errorf("msgs = %v, want success reply", sender.msgs)
	}
}

func TestDispatcherEditCaptionIgnored(t *testing.T) {
	sender := &recordingSender{}
	flow, _ := newFlow(t, sender, editQ(t))
	d := NewDispatcher(flow)
	startWithAnswer(t, flow, "x", 42)

	// Caption/media edit: EditedMessage.Text == "".
	d.Handle(context.Background(), sender, editedTextUpdate("", 5))

	if got := flow.Sessions.Get("x"); got.Answers[0].Answer != "A1" {
		t.Errorf("caption edit changed answer to %q", got.Answers[0].Answer)
	}
	if len(sender.msgs) != 0 {
		t.Errorf("caption edit produced replies: %v", sender.msgs)
	}
}

func TestDispatcherEditCommandIgnored(t *testing.T) {
	sender := &recordingSender{}
	flow, _ := newFlow(t, sender, editQ(t))
	d := NewDispatcher(flow)
	// id 42 matches the recorded answer, so only the IsCommand guard prevents a rewrite.
	startWithAnswer(t, flow, "x", 42)

	d.Handle(context.Background(), sender, editedCmdUpdate("/pull", 42))

	if got := flow.Sessions.Get("x"); got.Answers[0].Answer != "A1" {
		t.Errorf("edited command changed answer to %q", got.Answers[0].Answer)
	}
	if len(sender.msgs) != 0 {
		t.Errorf("edited command produced replies: %v", sender.msgs)
	}
}

func TestDispatcherEditSafetyGateReply(t *testing.T) {
	sender := &recordingSender{}
	flow, _ := newFlow(t, sender, editQ(t))
	d := NewDispatcher(flow)
	// Last answer carries a real id, so an unmatched edit is gated (no rewrite).
	startWithAnswer(t, flow, "x", 42)

	d.Handle(context.Background(), sender, editedTextUpdate("orphan edit", 9999))

	if got := flow.Sessions.Get("x"); got.Answers[0].Answer != "A1" {
		t.Errorf("safety gate failed: answer = %q, want A1", got.Answers[0].Answer)
	}
	if len(sender.msgs) == 0 || !strings.Contains(sender.msgs[len(sender.msgs)-1], "match") {
		t.Errorf("msgs = %v, want couldn't-match reply", sender.msgs)
	}
}
