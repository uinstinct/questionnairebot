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
	if err := m.RecordAnswer("daily", "Q1", "A1"); err != nil {
		t.Fatalf("RecordAnswer: %v", err)
	}
	got := m.Get("daily")
	if got.CurrentQuestionIndex != 1 || len(got.Answers) != 1 || got.Answers[0].Answer != "A1" {
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
			if err := m.RecordAnswer("daily", "Q", fmt.Sprintf("A%d", i)); err != nil {
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
