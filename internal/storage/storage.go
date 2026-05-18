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
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("write tmp: %w", err)
	}
	if len(existing) > 0 {
		if _, err := f.Write(existing); err != nil {
			f.Close()
			os.Remove(tmp)
			return fmt.Errorf("write tmp: %w", err)
		}
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("fsync tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}
