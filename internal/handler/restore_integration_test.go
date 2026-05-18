//go:build integration

package handler_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aditya-mitra/questionnairebot/internal/bot"
	"github.com/aditya-mitra/questionnairebot/internal/handler"
	"github.com/aditya-mitra/questionnairebot/internal/loader"
	"github.com/aditya-mitra/questionnairebot/internal/session"
)

type captureSender struct {
	mu       sync.Mutex
	msgs     []string
	markdown []string
}

func (c *captureSender) Send(text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, text)
	return nil
}

func (c *captureSender) SendMarkdown(text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.markdown = append(c.markdown, text)
	return nil
}

func (c *captureSender) SendPicker(text string, options []bot.PickerOption) error {
	return nil
}

func (c *captureSender) AckCallback(string) error { return nil }

func (c *captureSender) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := append([]string(nil), c.msgs...)
	return append(out, c.markdown...)
}

// TEST-05: after a crash mid-session, restore picks up at current_question_index.
func TestRestoreResumesFromMidSession(t *testing.T) {
	tmp := t.TempDir()
	loc := time.UTC

	q := &loader.Questionnaire{
		Slug: "daily", Name: "Daily", Schedule: "0 9 * * *", Timezone: "UTC", Location: loc,
		Questions: []loader.Question{
			{Question: "Q1?"},
			{Question: "Q2?", Example: "Ex2"},
			{Question: "Q3?"},
		},
	}

	// Pre-seed a partial session.yaml at index 1 (Q1 already answered).
	slugDir := filepath.Join(tmp, "daily")
	require.NoError(t, os.MkdirAll(slugDir, 0o755))
	sessYAML := "" +
		"questionnaire_id: daily\n" +
		"scheduled_for: 2026-05-18T09:00:00Z\n" +
		"started_at: 2026-05-18T09:00:00Z\n" +
		"current_question_index: 1\n" +
		"answers:\n" +
		"  - question: Q1?\n" +
		"    answer: A1\n"
	require.NoError(t, os.WriteFile(filepath.Join(slugDir, "session.yaml"), []byte(sessYAML), 0o644))

	sessions := session.NewManager(tmp)
	sender := &captureSender{}
	flow := handler.New(sender, sessions, tmp, []*loader.Questionnaire{q})

	require.NoError(t, handler.Restore(flow))

	loaded := sessions.Get("daily")
	require.NotNil(t, loaded, "session must be loaded into manager")
	require.Equal(t, 1, loaded.CurrentQuestionIndex)

	// Recording an answer (as the dispatcher would on free text) must send Q2 (index 1),
	// not Q1 (index 0). HandleAnswer sends the *next* question after recording, so seed
	// a SendQuestion at the restored index to mimic what the dispatcher does first.
	require.NoError(t, flow.SendQuestion("daily", loaded.CurrentQuestionIndex))

	all := sender.all()
	require.NotEmpty(t, all, "restore must surface the question at the resumed index")
	// Q2 has an example, so it goes via SendMarkdown.
	require.Contains(t, all[0], "Q2?", "first surfaced question must be Q2, not Q1")
	require.Contains(t, all[0], "_Example: Ex2_")
}
