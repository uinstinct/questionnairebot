package commands

import (
	"strings"
	"testing"
	"time"

	"github.com/aditya-mitra/questionnairebot/internal/handler"
	"github.com/aditya-mitra/questionnairebot/internal/loader"
	"github.com/aditya-mitra/questionnairebot/internal/session"
	"github.com/aditya-mitra/questionnairebot/internal/storage"
)

func setupPull(t *testing.T, qs []*loader.Questionnaire, now time.Time) (*Pull, *recordingSender, *session.Manager, string) {
	t.Helper()
	dir := t.TempDir()
	sessions := session.NewManager(dir)
	sender := &recordingSender{}
	flow := handler.New(sender, sessions, dir, qs)
	flow.Now = func() time.Time { return now }
	pull := NewPull(flow, flow.Now)
	return pull, sender, sessions, dir
}

func TestPullActiveSession(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	pull, sender, sessions, _ := setupPull(t, []*loader.Questionnaire{dailyQ()}, now)
	if _, err := sessions.Start("daily", now, now, time.UTC); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := pull.Handle(sender); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(sender.msgs) != 1 || sender.msgs[0] != ReplyActiveSession {
		t.Fatalf("msgs = %v", sender.msgs)
	}
}

func TestPullAllUpToDate(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	pull, sender, _, dir := setupPull(t, []*loader.Questionnaire{dailyQ()}, now)
	// Pre-seed: the most recent completed entry matches today's 9am cycle.
	t0 := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	if err := storage.PrependCompleted(dir, "daily", t0, t0.Add(15*time.Minute), time.UTC, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := pull.Handle(sender); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	// Next trigger after now=12:00 is tomorrow 9:00 (not yet completed).
	// So pending exists → picker shown, not "all up to date" — adjust expectation.
	// To force "all up to date" we need the next-upcoming cycle to already
	// match a completed entry. Easier: pre-seed completed entry for tomorrow's
	// cycle, simulating a fast-forward where the next cycle is already done.
	tomorrow := time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC)
	if err := storage.PrependCompleted(dir, "daily", tomorrow, tomorrow.Add(15*time.Minute), time.UTC, nil); err != nil {
		t.Fatalf("seed-future: %v", err)
	}
	sender.msgs = nil
	if err := pull.Handle(sender); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(sender.msgs) != 1 || sender.msgs[0] != ReplyAllUpToDate {
		t.Fatalf("expected all-up-to-date, got %v", sender.msgs)
	}
}

func TestPullPastDueAddsSkipsAndShowsPicker(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	pull, sender, _, dir := setupPull(t, []*loader.Questionnaire{dailyQ()}, now)
	// Baseline: a completed entry 3 days ago. Daily cron @ 9:00, so we expect
	// skips prepended for the 16th, 17th, and 18th (9:00 each), then next-upcoming
	// is the 19th 09:00.
	baseline := time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC)
	if err := storage.PrependCompleted(dir, "daily", baseline, baseline.Add(15*time.Minute), time.UTC, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := pull.Handle(sender); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	last, err := storage.LastEntry(dir, "daily")
	if err != nil {
		t.Fatalf("LastEntry: %v", err)
	}
	if last == nil || last.Status != "skipped" {
		t.Fatalf("expected newest entry to be skipped, got %+v", last)
	}
	if len(sender.pickers) != 1 || len(sender.pickers[0].Options) != 1 {
		t.Fatalf("picker = %+v", sender.pickers)
	}
	if !strings.HasPrefix(sender.pickers[0].Options[0].CallbackData, "start:daily:") {
		t.Errorf("callback data = %q", sender.pickers[0].Options[0].CallbackData)
	}
}

func TestPullCallbackStartsSession(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	pull, sender, sessions, _ := setupPull(t, []*loader.Questionnaire{dailyQ()}, now)
	tomorrow := time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC)
	cbData := "start:daily:" + tomorrow.UTC().Format(time.RFC3339)
	if err := pull.HandleCallback(sender, cbData); err != nil {
		t.Fatalf("HandleCallback: %v", err)
	}
	if sessions.Get("daily") == nil {
		t.Fatalf("session should be active")
	}
	if len(sender.msgs) != 1 || sender.msgs[0] != "Q1?" {
		t.Fatalf("msgs = %v", sender.msgs)
	}
}

func TestPullCallbackMalformed(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	pull, sender, _, _ := setupPull(t, []*loader.Questionnaire{dailyQ()}, now)
	if err := pull.HandleCallback(sender, "bogus"); err != nil {
		t.Fatalf("HandleCallback: %v", err)
	}
	if len(sender.msgs) != 1 || sender.msgs[0] != ReplyBadCallback {
		t.Fatalf("msgs = %v", sender.msgs)
	}
}
