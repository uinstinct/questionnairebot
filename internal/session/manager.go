// Package session manages in-progress questionnaire sessions.
//
// A single Manager owns an in-memory map of slug -> *Session, guarded by one
// sync.Mutex. All disk writes happen under the lock so cron and Telegram
// polling goroutines cannot race.
package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Manager owns the in-memory map of active sessions, persisting each change to
// disk under a single mutex.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	dataDir  string
}

// NewManager constructs a Manager rooted at dataDir.
func NewManager(dataDir string) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		dataDir:  dataDir,
	}
}

// Start creates a new session for slug, persists it, and returns a copy.
func (m *Manager) Start(slug string, scheduled, started time.Time, loc *time.Location) (*Session, error) {
	if loc == nil {
		return nil, errors.New("session: loc is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := &Session{
		QuestionnaireID:      slug,
		ScheduledFor:         scheduled.In(loc).Format(time.RFC3339),
		StartedAt:            started.In(loc).Format(time.RFC3339),
		CurrentQuestionIndex: 0,
	}
	m.sessions[slug] = s
	if err := m.saveLocked(slug); err != nil {
		delete(m.sessions, slug)
		return nil, err
	}
	return cloneSession(s), nil
}

// Get returns a copy of the active session for slug, or nil if none exists.
func (m *Manager) Get(slug string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[slug]
	if !ok {
		return nil
	}
	return cloneSession(s)
}

// RecordAnswer appends an answer to the active session for slug and persists.
func (m *Manager) RecordAnswer(slug, question, answer string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[slug]
	if !ok {
		return fmt.Errorf("session: no active session for %q", slug)
	}
	s.Answers = append(s.Answers, AnswerPair{Question: question, Answer: answer})
	s.CurrentQuestionIndex++
	return m.saveLocked(slug)
}

// Delete removes the active session for slug from memory and disk.
func (m *Manager) Delete(slug string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, slug)
	if err := os.Remove(sessionPath(m.dataDir, slug)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// LoadFromDisk rehydrates a persisted session for slug into the manager.
// Returns nil if no session file exists.
func (m *Manager) LoadFromDisk(slug string) (*Session, error) {
	raw, err := os.ReadFile(sessionPath(m.dataDir, slug))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var s Session
	if err := yaml.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("decode session.yaml: %w", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[slug] = &s
	return cloneSession(&s), nil
}

func (m *Manager) saveLocked(slug string) error {
	s, ok := m.sessions[slug]
	if !ok {
		return fmt.Errorf("session: no session for %q", slug)
	}
	path := sessionPath(m.dataDir, slug)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", tmp, err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write tmp: %w", err)
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
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

func sessionPath(dataDir, slug string) string {
	return filepath.Join(dataDir, slug, "session.yaml")
}

func cloneSession(s *Session) *Session {
	if s == nil {
		return nil
	}
	cp := *s
	if s.Answers != nil {
		cp.Answers = append([]AnswerPair(nil), s.Answers...)
	}
	return &cp
}
