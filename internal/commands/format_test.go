package commands

import (
	"strings"
	"testing"
	"time"

	"github.com/aditya-mitra/questionnairebot/internal/loader"
	"github.com/aditya-mitra/questionnairebot/internal/session"
	"github.com/aditya-mitra/questionnairebot/internal/storage"
)

func TestListIncludesAllFields(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	qs := map[string]*loader.Questionnaire{
		"daily":  {Slug: "daily", Name: "Daily", Schedule: "0 9 * * *", Timezone: "Asia/Kolkata", Location: loc},
		"weekly": {Slug: "weekly", Name: "Weekly", Schedule: "0 10 * * 1", Timezone: "UTC", Location: time.UTC},
	}
	l := NewList(qs, func() time.Time { return time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC) })
	out := l.Render()
	if !strings.Contains(out, "📋 Questionnaires:") {
		t.Errorf("missing header: %s", out)
	}
	if !strings.Contains(out, "Daily | cron=0 9 * * * | tz=Asia/Kolkata | next=") {
		t.Errorf("missing Daily row: %s", out)
	}
	if !strings.Contains(out, "Weekly | cron=0 10 * * 1 | tz=UTC | next=") {
		t.Errorf("missing Weekly row: %s", out)
	}
	// Slug-sorted: daily before weekly.
	if strings.Index(out, "Daily ") > strings.Index(out, "Weekly ") {
		t.Errorf("rows not sorted by slug: %s", out)
	}
}

func TestStatusStates(t *testing.T) {
	dir := t.TempDir()
	sessions := session.NewManager(dir)
	loc := time.UTC
	qs := map[string]*loader.Questionnaire{
		"a-active":    {Slug: "a-active", Name: "Active", Schedule: "0 9 * * *", Timezone: "UTC", Location: loc, Questions: []loader.Question{{Question: "Q?"}}},
		"b-done":      {Slug: "b-done", Name: "Done", Schedule: "0 9 * * *", Timezone: "UTC", Location: loc, Questions: []loader.Question{{Question: "Q?"}}},
		"c-pending":   {Slug: "c-pending", Name: "Pending", Schedule: "0 9 * * *", Timezone: "UTC", Location: loc, Questions: []loader.Question{{Question: "Q?"}}},
	}
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, loc)
	// Active session.
	if _, err := sessions.Start("a-active", now, now, loc); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Completed entry for b-done.
	if err := storage.PrependCompleted(dir, "b-done", now.Add(-3*time.Hour), now.Add(-2*time.Hour), loc, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}

	s := NewStatus(dir, sessions, qs, func() time.Time { return now })
	out := s.Render()
	if !strings.Contains(out, "📊 Status:") {
		t.Errorf("missing header: %s", out)
	}
	if !strings.Contains(out, "Active |") || !strings.Contains(out, "🔄 In Progress") {
		t.Errorf("missing active row: %s", out)
	}
	if !strings.Contains(out, "Done |") || !strings.Contains(out, "✅ Done") {
		t.Errorf("missing done row: %s", out)
	}
	if !strings.Contains(out, "Pending |") || !strings.Contains(out, "⏳ Pending") {
		t.Errorf("missing pending row: %s", out)
	}
	if !strings.Contains(out, "last=Never") {
		t.Errorf("expected last=Never for pending row: %s", out)
	}
}
