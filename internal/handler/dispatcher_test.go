package handler

import (
	"context"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

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
