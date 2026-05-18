package commands

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aditya-mitra/questionnairebot/internal/handler"
	"github.com/aditya-mitra/questionnairebot/internal/loader"
	"github.com/aditya-mitra/questionnairebot/internal/session"
	"github.com/aditya-mitra/questionnairebot/internal/storage"
)

func dailyQ() *loader.Questionnaire {
	return &loader.Questionnaire{
		Slug: "daily", Name: "Daily", Schedule: "0 9 * * *", Timezone: "UTC", Location: time.UTC,
		Questions: []loader.Question{{Question: "Q1?"}},
	}
}

func weeklyQ() *loader.Questionnaire {
	return &loader.Questionnaire{
		Slug: "weekly", Name: "Weekly", Schedule: "0 10 * * 1", Timezone: "UTC", Location: time.UTC,
		Questions: []loader.Question{{Question: "Wk?"}},
	}
}

func setupBus(t *testing.T, qs []*loader.Questionnaire) (*CronBus, *recordingSender, *session.Manager, string) {
	t.Helper()
	dir := t.TempDir()
	sessions := session.NewManager(dir)
	sender := &recordingSender{}
	flow := handler.New(sender, sessions, dir, qs)
	flow.Now = func() time.Time { return time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC) }
	bus := NewCronBus(flow, sender, flow.Now)
	bus.Window = 30 * time.Millisecond
	return bus, sender, sessions, dir
}

func waitForFlush(bus *CronBus) {
	time.Sleep(bus.Window + 50*time.Millisecond)
}

func TestCronSingleFireStartsSession(t *testing.T) {
	bus, sender, sessions, _ := setupBus(t, []*loader.Questionnaire{dailyQ()})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go bus.Run(ctx)

	bus.Fire("daily", time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC))
	waitForFlush(bus)

	if sessions.Get("daily") == nil {
		t.Fatalf("session not started")
	}
	msgs, _, pickers := sender.snapshot()
	if len(msgs) != 1 || msgs[0] != "Q1?" {
		t.Fatalf("msgs = %v", msgs)
	}
	if len(pickers) != 0 {
		t.Errorf("unexpected picker: %+v", pickers)
	}
}

func TestCronAlreadyCompletedSilent(t *testing.T) {
	bus, sender, sessions, dir := setupBus(t, []*loader.Questionnaire{dailyQ()})
	t0 := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	// Pre-seed answers.yaml with a completed entry matching the fire time.
	if err := storage.PrependCompleted(dir, "daily", t0, t0.Add(15*time.Minute), time.UTC, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go bus.Run(ctx)

	bus.Fire("daily", t0)
	waitForFlush(bus)

	if sessions.Get("daily") != nil {
		t.Errorf("session should NOT start when cycle already completed")
	}
	msgs, _, pickers := sender.snapshot()
	if len(msgs) != 0 || len(pickers) != 0 {
		t.Errorf("expected silence, got msgs=%v pickers=%v", msgs, pickers)
	}
}

func TestCronMultiFirePicker(t *testing.T) {
	bus, sender, sessions, _ := setupBus(t, []*loader.Questionnaire{dailyQ(), weeklyQ()})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go bus.Run(ctx)

	t0 := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	bus.Fire("daily", t0)
	bus.Fire("weekly", t0)
	waitForFlush(bus)

	if sessions.Get("daily") != nil || sessions.Get("weekly") != nil {
		t.Errorf("no session should auto-start in multi-fire case")
	}
	_, _, pickers := sender.snapshot()
	if len(pickers) != 1 {
		t.Fatalf("expected 1 picker, got %d (%+v)", len(pickers), pickers)
	}
	p := pickers[0]
	if !strings.Contains(p.Text, "📋 Multiple questionnaires are due.") {
		t.Errorf("picker text = %q", p.Text)
	}
	if len(p.Options) != 2 {
		t.Errorf("picker options = %v", p.Options)
	}
}

func TestCronActiveSessionSkips(t *testing.T) {
	bus, sender, sessions, _ := setupBus(t, []*loader.Questionnaire{dailyQ()})
	// Start a session manually.
	t0 := time.Date(2026, 5, 18, 8, 0, 0, 0, time.UTC)
	if _, err := sessions.Start("daily", t0, t0, time.UTC); err != nil {
		t.Fatalf("Start: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go bus.Run(ctx)

	bus.Fire("daily", time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC))
	waitForFlush(bus)

	msgs, _, pickers := sender.snapshot()
	if len(msgs) != 0 || len(pickers) != 0 {
		t.Errorf("expected silence when session active, got msgs=%v pickers=%v", msgs, pickers)
	}
}
