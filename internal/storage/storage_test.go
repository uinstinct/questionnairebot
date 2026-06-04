package storage

import (
	"os"
	"path/filepath"
	"strings"
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

func seedCompleted(t *testing.T, dir, slug string, answers []AnswerPair) {
	t.Helper()
	loc := time.UTC
	t0 := time.Date(2026, 5, 18, 9, 0, 0, 0, loc)
	if err := PrependCompleted(dir, slug, t0, t0.Add(15*time.Minute), loc, answers); err != nil {
		t.Fatalf("PrependCompleted: %v", err)
	}
}

func readEntries(t *testing.T, dir, slug string) []Entry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, slug, "answers.yaml"))
	if err != nil {
		t.Fatalf("read answers.yaml: %v", err)
	}
	var entries []Entry
	if err := yaml.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return entries
}

func TestUpdateAnswerByMessageIDMatch(t *testing.T) {
	dir := t.TempDir()
	// entry A seeded first -> ends up at entries[1]; entry B -> entries[0].
	seedCompleted(t, dir, "daily", []AnswerPair{
		{Question: "Q1?", Answer: "A1", MessageID: 10},
		{Question: "Q2?", Answer: "A2", MessageID: 11},
	})
	seedCompleted(t, dir, "daily", []AnswerPair{
		{Question: "Q1?", Answer: "B1", MessageID: 20},
		{Question: "Q2?", Answer: "B2", MessageID: 21},
	})

	matched, err := UpdateAnswerByMessageID(dir, "daily", 11, "edited A2")
	if err != nil {
		t.Fatalf("UpdateAnswerByMessageID: %v", err)
	}
	if !matched {
		t.Fatalf("matched = false, want true")
	}

	entries := readEntries(t, dir, "daily")
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	// Ordering + statuses preserved.
	if entries[0].Status != "completed" || entries[1].Status != "completed" {
		t.Fatalf("statuses = %q / %q", entries[0].Status, entries[1].Status)
	}
	// Target answer changed.
	if got := entries[1].Answers[1].Answer; got != "edited A2" {
		t.Errorf("target answer = %q, want %q", got, "edited A2")
	}
	// MessageID preserved on the edited answer.
	if got := entries[1].Answers[1].MessageID; got != 11 {
		t.Errorf("target message_id = %d, want 11", got)
	}
	// Sibling in same entry untouched.
	if got := entries[1].Answers[0].Answer; got != "A1" {
		t.Errorf("sibling answer = %q, want A1", got)
	}
	// Other entry untouched.
	if entries[0].Answers[0].Answer != "B1" || entries[0].Answers[1].Answer != "B2" {
		t.Errorf("other entry answers = %+v", entries[0].Answers)
	}
}

func TestUpdateAnswerByMessageIDNoMatch(t *testing.T) {
	dir := t.TempDir()
	seedCompleted(t, dir, "daily", []AnswerPair{{Question: "Q1?", Answer: "A1", MessageID: 10}})
	before, err := os.ReadFile(filepath.Join(dir, "daily", "answers.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	matched, err := UpdateAnswerByMessageID(dir, "daily", 999, "nope")
	if err != nil {
		t.Fatalf("UpdateAnswerByMessageID: %v", err)
	}
	if matched {
		t.Fatalf("matched = true, want false")
	}
	after, err := os.ReadFile(filepath.Join(dir, "daily", "answers.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("file rewritten on no-match:\nbefore=%q\nafter=%q", before, after)
	}
}

func TestUpdateAnswerByMessageIDMissingFile(t *testing.T) {
	dir := t.TempDir()
	matched, err := UpdateAnswerByMessageID(dir, "missing", 10, "x")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if matched {
		t.Fatalf("matched = true, want false")
	}
}

func TestUpdateAnswerByMessageIDZeroNeverMatches(t *testing.T) {
	dir := t.TempDir()
	// Legacy answer with no stored message id (MessageID == 0).
	seedCompleted(t, dir, "daily", []AnswerPair{{Question: "Q1?", Answer: "A1"}})
	before, err := os.ReadFile(filepath.Join(dir, "daily", "answers.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	matched, err := UpdateAnswerByMessageID(dir, "daily", 0, "clobber")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if matched {
		t.Fatalf("matched = true, want false (id 0 must never match a legacy answer)")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "daily", "answers.yaml"))
	if string(before) != string(after) {
		t.Errorf("legacy answer clobbered by id-0 edit")
	}
}

func TestUpdateLastAnswerLegacyRewrites(t *testing.T) {
	dir := t.TempDir()
	// entries[1] (older) has a real id; entries[0] (newest) is legacy.
	seedCompleted(t, dir, "daily", []AnswerPair{{Question: "Q1?", Answer: "old", MessageID: 5}})
	seedCompleted(t, dir, "daily", []AnswerPair{
		{Question: "Q1?", Answer: "first", MessageID: 7},
		{Question: "Q2?", Answer: "legacy"},
	})

	matched, err := UpdateLastAnswer(dir, "daily", "corrected")
	if err != nil {
		t.Fatalf("UpdateLastAnswer: %v", err)
	}
	if !matched {
		t.Fatalf("matched = false, want true")
	}
	entries := readEntries(t, dir, "daily")
	if got := entries[0].Answers[1].Answer; got != "corrected" {
		t.Errorf("last answer = %q, want corrected", got)
	}
	// Sibling with a real id in entries[0] untouched.
	if entries[0].Answers[0].Answer != "first" {
		t.Errorf("sibling answer = %q, want first", entries[0].Answers[0].Answer)
	}
	// Older entry untouched.
	if entries[1].Answers[0].Answer != "old" {
		t.Errorf("older entry answer = %q, want old", entries[1].Answers[0].Answer)
	}
}

func TestUpdateLastAnswerRealIDGated(t *testing.T) {
	dir := t.TempDir()
	seedCompleted(t, dir, "daily", []AnswerPair{{Question: "Q1?", Answer: "real", MessageID: 42}})
	before, err := os.ReadFile(filepath.Join(dir, "daily", "answers.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	matched, err := UpdateLastAnswer(dir, "daily", "should-not-apply")
	if err != nil {
		t.Fatalf("UpdateLastAnswer: %v", err)
	}
	if matched {
		t.Fatalf("matched = true, want false (real-id last answer must be gated)")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "daily", "answers.yaml"))
	if string(before) != string(after) {
		t.Errorf("file rewritten despite safety gate")
	}
}

func TestUpdateLastAnswerEdgeCases(t *testing.T) {
	// Missing file.
	dir := t.TempDir()
	if matched, err := UpdateLastAnswer(dir, "missing", "x"); err != nil || matched {
		t.Fatalf("missing file = (%v, %v), want (false, nil)", matched, err)
	}

	// Empty entries: a YAML file decoding to an empty sequence.
	emptyDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(emptyDir, "daily"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(emptyDir, "daily", "answers.yaml"), []byte("[]\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if matched, err := UpdateLastAnswer(emptyDir, "daily", "x"); err != nil || matched {
		t.Fatalf("empty entries = (%v, %v), want (false, nil)", matched, err)
	}

	// entries[0] with no answers (a skipped entry).
	skipDir := t.TempDir()
	loc := time.UTC
	t0 := time.Date(2026, 5, 18, 9, 0, 0, 0, loc)
	if err := PrependSkipped(skipDir, "daily", t0, t0, loc); err != nil {
		t.Fatalf("PrependSkipped: %v", err)
	}
	if matched, err := UpdateLastAnswer(skipDir, "daily", "x"); err != nil || matched {
		t.Fatalf("entries[0] no answers = (%v, %v), want (false, nil)", matched, err)
	}
}

func TestLegacyRoundTripOmitsMessageID(t *testing.T) {
	dir := t.TempDir()
	// Legacy file written without any message_id.
	seedCompleted(t, dir, "daily", []AnswerPair{
		{Question: "Q1?", Answer: "A1"},
		{Question: "Q2?", Answer: "A2"},
	})
	raw, err := os.ReadFile(filepath.Join(dir, "daily", "answers.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(raw), "message_id") {
		t.Fatalf("seeded legacy file unexpectedly contains message_id: %q", raw)
	}
	// Decodes to MessageID == 0.
	entries := readEntries(t, dir, "daily")
	for i, a := range entries[0].Answers {
		if a.MessageID != 0 {
			t.Errorf("answers[%d].MessageID = %d, want 0", i, a.MessageID)
		}
	}
	// A full re-marshal through the edit core must not emit "message_id: 0".
	matched, err := UpdateLastAnswer(dir, "daily", "corrected")
	if err != nil || !matched {
		t.Fatalf("UpdateLastAnswer = (%v, %v), want (true, nil)", matched, err)
	}
	rewritten, err := os.ReadFile(filepath.Join(dir, "daily", "answers.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(rewritten), "message_id: 0") {
		t.Errorf("re-marshalled file contains \"message_id: 0\": %q", rewritten)
	}
}
