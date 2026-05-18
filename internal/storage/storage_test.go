package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestPrependPreservesOrder(t *testing.T) {
	dir := t.TempDir()
	slug := "daily"
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	t0 := time.Date(2026, 5, 18, 9, 0, 0, 0, loc)
	if err := PrependCompleted(dir, slug, t0, t0.Add(15*time.Minute), loc, []AnswerPair{{Question: "Q1?", Answer: "A1"}}); err != nil {
		t.Fatalf("PrependCompleted: %v", err)
	}
	if err := PrependSkipped(dir, slug, t0.Add(-24*time.Hour), t0, loc); err != nil {
		t.Fatalf("PrependSkipped: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, slug, "answers.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var entries []Entry
	if err := yaml.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d (raw=%q)", len(entries), raw)
	}
	if entries[0].Status != "skipped" {
		t.Errorf("entry[0].Status = %q, want skipped", entries[0].Status)
	}
	if entries[1].Status != "completed" {
		t.Errorf("entry[1].Status = %q, want completed", entries[1].Status)
	}

	last, err := LastEntry(dir, slug)
	if err != nil {
		t.Fatalf("LastEntry: %v", err)
	}
	if last == nil || last.Status != "skipped" {
		t.Errorf("LastEntry status = %v, want skipped", last)
	}

	missing, err := LastEntry(dir, "no-such-slug")
	if err != nil {
		t.Errorf("LastEntry missing: %v", err)
	}
	if missing != nil {
		t.Errorf("LastEntry missing = %+v, want nil", missing)
	}
}

func TestPrependMany(t *testing.T) {
	dir := t.TempDir()
	loc := time.UTC
	t0 := time.Now().In(loc)
	for i := 0; i < 100; i++ {
		if err := PrependCompleted(dir, "x", t0.Add(time.Duration(i)*time.Hour), t0.Add(time.Duration(i)*time.Hour+time.Minute), loc, nil); err != nil {
			t.Fatalf("PrependCompleted %d: %v", i, err)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "x", "answers.yaml"))
	var entries []Entry
	if err := yaml.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries) != 100 {
		t.Fatalf("want 100 entries, got %d", len(entries))
	}
}
