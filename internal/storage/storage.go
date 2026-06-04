// Package storage writes questionnaire answer history to disk.
//
// Entries are prepended (newest first) to data/<slug>/answers.yaml using an
// atomic temp-file + rename. The file is never rewritten from scratch.
//
// PrependCompleted/PrependSkipped are NOT goroutine-safe across writers; the
// caller (session-finalizer in Phase 3) is responsible for serialising writes
// per slug.
package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// PrependCompleted writes a "completed" entry to the head of data/<slug>/answers.yaml.
func PrependCompleted(dataDir, slug string, scheduled, completed time.Time, loc *time.Location, answers []AnswerPair) error {
	if loc == nil {
		return errors.New("storage: loc is required")
	}
	entry := Entry{
		Status:       "completed",
		ScheduledFor: scheduled.In(loc).Format(time.RFC3339),
		CompletedAt:  completed.In(loc).Format(time.RFC3339),
		Answers:      answers,
	}
	return prepend(answersPath(dataDir, slug), entry)
}

// PrependSkipped writes a "skipped" entry to the head of data/<slug>/answers.yaml.
func PrependSkipped(dataDir, slug string, scheduled, skipped time.Time, loc *time.Location) error {
	if loc == nil {
		return errors.New("storage: loc is required")
	}
	entry := Entry{
		Status:       "skipped",
		ScheduledFor: scheduled.In(loc).Format(time.RFC3339),
		SkippedAt:    skipped.In(loc).Format(time.RFC3339),
	}
	return prepend(answersPath(dataDir, slug), entry)
}

// LastEntry returns the most recent entry from data/<slug>/answers.yaml,
// or nil if the file does not yet exist or contains no entries.
func LastEntry(dataDir, slug string) (*Entry, error) {
	raw, err := os.ReadFile(answersPath(dataDir, slug))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var entries []Entry
	if err := yaml.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("decode answers.yaml: %w", err)
	}
	if len(entries) == 0 {
		return nil, nil
	}
	return &entries[0], nil
}

// UpdateAnswerByMessageID rewrites data/<slug>/answers.yaml in place, setting the
// answer text of the FIRST AnswerPair whose MessageID == messageID. Returns
// matched=false (no error) when messageID is 0, the file is absent, or no answer
// carries that id. Telegram message ids are always > 0, so messageID == 0 is
// rejected up front and can never match a legacy (id-less) answer.
func UpdateAnswerByMessageID(dataDir, slug string, messageID int, newText string) (matched bool, err error) {
	if messageID == 0 {
		return false, nil
	}
	return updateEntries(dataDir, slug, func(entries []Entry) bool {
		for i := range entries {
			for j := range entries[i].Answers {
				if entries[i].Answers[j].MessageID == messageID {
					entries[i].Answers[j].Answer = newText
					return true
				}
			}
		}
		return false
	})
}

// UpdateLastAnswer rewrites the last AnswerPair of the newest entry (entries[0])
// of data/<slug>/answers.yaml — but ONLY when that answer is legacy data with no
// stored message id (MessageID == 0). This is the backwards-compatibility
// fallback for the edit feature: a most-recent answer that already carries a real
// message id is deliberately left untouched (returns false) so an unmatched edit
// can never clobber a genuine answer. It is NOT an unconditional last-answer
// setter. Returns (false, nil) when the file is absent, has no entries, or
// entries[0] has no answers.
func UpdateLastAnswer(dataDir, slug string, newText string) (matched bool, err error) {
	return updateEntries(dataDir, slug, func(entries []Entry) bool {
		if len(entries) == 0 || len(entries[0].Answers) == 0 {
			return false
		}
		last := len(entries[0].Answers) - 1
		if entries[0].Answers[last].MessageID != 0 {
			return false
		}
		entries[0].Answers[last].Answer = newText
		return true
	})
}

// updateEntries reads data/<slug>/answers.yaml, decodes it into []Entry, and
// applies mutate. ONLY when mutate reports a change does it re-marshal the WHOLE
// document and write it atomically via the same temp-file + fsync + rename
// ceremony as prepend (full re-marshal, not prepend's byte-concatenation). A
// missing file is reported as (false, nil).
func updateEntries(dataDir, slug string, mutate func(entries []Entry) bool) (bool, error) {
	path := answersPath(dataDir, slug)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	var entries []Entry
	if err := yaml.Unmarshal(raw, &entries); err != nil {
		return false, fmt.Errorf("decode answers.yaml: %w", err)
	}
	if !mutate(entries) {
		return false, nil
	}

	encoded, err := yaml.Marshal(entries)
	if err != nil {
		return false, fmt.Errorf("marshal entries: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return false, fmt.Errorf("open %s: %w", tmp, err)
	}
	if _, err := f.Write(encoded); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return false, fmt.Errorf("write tmp: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return false, fmt.Errorf("fsync tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return false, fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return false, fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return true, nil
}

func answersPath(dataDir, slug string) string {
	return filepath.Join(dataDir, slug, "answers.yaml")
}

func prepend(path string, e Entry) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	encoded, err := yaml.Marshal([]Entry{e})
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", path, err)
	}

	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", tmp, err)
	}
	if _, err := f.Write(encoded); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write tmp: %w", err)
	}
	if len(existing) > 0 {
		if _, err := f.Write(existing); err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return fmt.Errorf("write tmp: %w", err)
		}
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("fsync tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}
