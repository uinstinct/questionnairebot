package session

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestManagerLifecycle(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	loc := time.UTC
	t0 := time.Date(2026, 5, 18, 9, 0, 0, 0, loc)

	if _, err := m.Start("daily", t0, t0, loc); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got := m.Get("daily"); got == nil || got.CurrentQuestionIndex != 0 {
		t.Fatalf("Get after Start = %+v", got)
	}
	if err := m.RecordAnswer("daily", "Q1", "A1", 101); err != nil {
		t.Fatalf("RecordAnswer: %v", err)
	}
	got := m.Get("daily")
	if got.CurrentQuestionIndex != 1 || len(got.Answers) != 1 || got.Answers[0].Answer != "A1" || got.Answers[0].MessageID != 101 {
		t.Fatalf("after RecordAnswer = %+v", got)
	}

	// Reload from disk.
	m2 := NewManager(dir)
	loaded, err := m2.LoadFromDisk("daily")
	if err != nil {
		t.Fatalf("LoadFromDisk: %v", err)
	}
	if loaded.CurrentQuestionIndex != 1 || len(loaded.Answers) != 1 {
		t.Fatalf("loaded = %+v", loaded)
	}

	if err := m.Delete("daily"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := m.Delete("daily"); err != nil {
		t.Fatalf("Delete twice: %v", err)
	}
	if m.Get("daily") != nil {
		t.Errorf("Get after Delete should be nil")
	}

	none, err := m.LoadFromDisk("nope")
	if err != nil || none != nil {
		t.Errorf("LoadFromDisk missing: %v, %v", none, err)
	}
}

func TestManagerConcurrent(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	loc := time.UTC
	t0 := time.Now().In(loc)
	if _, err := m.Start("daily", t0, t0, loc); err != nil {
		t.Fatalf("Start: %v", err)
	}

	var wg sync.WaitGroup
	const N = 50
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			if err := m.RecordAnswer("daily", "Q", fmt.Sprintf("A%d", i), i+1); err != nil {
				t.Errorf("RecordAnswer %d: %v", i, err)
			}
		}()
	}
	wg.Wait()

	got := m.Get("daily")
	if got.CurrentQuestionIndex != N {
		t.Fatalf("CurrentQuestionIndex = %d, want %d", got.CurrentQuestionIndex, N)
	}

	m2 := NewManager(dir)
	loaded, err := m2.LoadFromDisk("daily")
	if err != nil {
		t.Fatalf("LoadFromDisk: %v", err)
	}
	if loaded.CurrentQuestionIndex != N {
		t.Fatalf("loaded CurrentQuestionIndex = %d, want %d", loaded.CurrentQuestionIndex, N)
	}
}

func TestRecordAnswerPersistsMessageID(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	loc := time.UTC
	t0 := time.Date(2026, 5, 18, 9, 0, 0, 0, loc)
	if _, err := m.Start("daily", t0, t0, loc); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := m.RecordAnswer("daily", "Q1", "A1", 55); err != nil {
		t.Fatalf("RecordAnswer: %v", err)
	}

	loaded, err := NewManager(dir).LoadFromDisk("daily")
	if err != nil {
		t.Fatalf("LoadFromDisk: %v", err)
	}
	if len(loaded.Answers) != 1 || loaded.Answers[0].MessageID != 55 {
		t.Fatalf("reloaded answers = %+v, want one answer with message_id 55", loaded.Answers)
	}
}

func TestManagerUpdateAnswerByMessageID(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	loc := time.UTC
	t0 := time.Date(2026, 5, 18, 9, 0, 0, 0, loc)
	if _, err := m.Start("daily", t0, t0, loc); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := m.RecordAnswer("daily", "Q1", "A1", 10); err != nil {
		t.Fatalf("RecordAnswer 1: %v", err)
	}
	if err := m.RecordAnswer("daily", "Q2", "A2", 20); err != nil {
		t.Fatalf("RecordAnswer 2: %v", err)
	}

	ok, err := m.UpdateAnswerByMessageID("daily", 20, "edited A2")
	if err != nil || !ok {
		t.Fatalf("UpdateAnswerByMessageID match = (%v, %v), want (true, nil)", ok, err)
	}
	got := m.Get("daily")
	if got.Answers[1].Answer != "edited A2" || got.Answers[0].Answer != "A1" {
		t.Fatalf("in-memory answers = %+v", got.Answers)
	}
	// Persisted to disk under the lock.
	loaded, err := NewManager(dir).LoadFromDisk("daily")
	if err != nil {
		t.Fatalf("LoadFromDisk: %v", err)
	}
	if loaded.Answers[1].Answer != "edited A2" {
		t.Errorf("reloaded answer = %q, want edited A2", loaded.Answers[1].Answer)
	}

	// Unmatched id, messageID 0, and missing session all no-op.
	if ok, err := m.UpdateAnswerByMessageID("daily", 999, "no"); err != nil || ok {
		t.Errorf("unmatched id = (%v, %v), want (false, nil)", ok, err)
	}
	if ok, err := m.UpdateAnswerByMessageID("daily", 0, "no"); err != nil || ok {
		t.Errorf("messageID 0 = (%v, %v), want (false, nil)", ok, err)
	}
	if ok, err := m.UpdateAnswerByMessageID("absent", 10, "no"); err != nil || ok {
		t.Errorf("missing session = (%v, %v), want (false, nil)", ok, err)
	}
}

func TestManagerUpdateLastAnswerLegacyGate(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	loc := time.UTC
	t0 := time.Date(2026, 5, 18, 9, 0, 0, 0, loc)
	if _, err := m.Start("daily", t0, t0, loc); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// No answers yet -> no-op.
	if ok, err := m.UpdateLastAnswer("daily", "x"); err != nil || ok {
		t.Fatalf("empty answers = (%v, %v), want (false, nil)", ok, err)
	}
	// Missing session -> no-op.
	if ok, err := m.UpdateLastAnswer("absent", "x"); err != nil || ok {
		t.Fatalf("missing session = (%v, %v), want (false, nil)", ok, err)
	}
	// Legacy last answer (MessageID == 0) -> rewritten.
	if err := m.RecordAnswer("daily", "Q1", "legacy", 0); err != nil {
		t.Fatalf("RecordAnswer legacy: %v", err)
	}
	if ok, err := m.UpdateLastAnswer("daily", "fixed"); err != nil || !ok {
		t.Fatalf("legacy last answer = (%v, %v), want (true, nil)", ok, err)
	}
	if got := m.Get("daily"); got.Answers[0].Answer != "fixed" {
		t.Errorf("after legacy rewrite = %q, want fixed", got.Answers[0].Answer)
	}
	// Real-id last answer -> gated.
	if err := m.RecordAnswer("daily", "Q2", "real", 77); err != nil {
		t.Fatalf("RecordAnswer real: %v", err)
	}
	if ok, err := m.UpdateLastAnswer("daily", "nope"); err != nil || ok {
		t.Fatalf("real-id last answer = (%v, %v), want (false, nil)", ok, err)
	}
	if got := m.Get("daily"); got.Answers[1].Answer != "real" {
		t.Errorf("gated answer changed to %q, want real", got.Answers[1].Answer)
	}
}
