//go:build integration

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/aditya-mitra/questionnairebot/internal/storage"
)

// TEST-03: cron fires → bot sends Q1 → user answers → … → completion → answers.yaml has a completed entry.
func TestE2EHappyPath(t *testing.T) {
	token, chatID := requireTestEnv(t)

	dir := t.TempDir()
	slug := "daily"
	slugDir := filepath.Join(dir, slug)
	require.NoError(t, os.MkdirAll(slugDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(slugDir, "questionnaire.yaml"), []byte(""+
		"name: Daily\n"+
		"schedule: 0 9 * * *\n"+
		"timezone: UTC\n"+
		"questions:\n"+
		"  - question: How was today?\n"+
		"  - question: Anything else?\n"+
		"    example: A detail\n"), 0o644))

	rig, teardown := newBotUnderTest(t, dir, token, chatID)
	defer teardown()

	probe := newProbeClient(t, rig)

	rig.bus.Fire(slug, time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC))

	probe.waitForMessage(t, 15*time.Second, func(m tgbotapi.Message) bool {
		return strings.Contains(m.Text, "How was today?")
	})
	probe.send(t, "fine")

	probe.waitForMessage(t, 15*time.Second, func(m tgbotapi.Message) bool {
		return strings.Contains(m.Text, "Anything else?")
	})
	probe.send(t, "nope")

	probe.waitForMessage(t, 15*time.Second, func(m tgbotapi.Message) bool {
		return strings.Contains(m.Text, "Daily complete!")
	})

	// Verify answers.yaml on disk.
	raw, err := os.ReadFile(filepath.Join(slugDir, "answers.yaml"))
	require.NoError(t, err)
	var entries []storage.Entry
	require.NoError(t, yaml.Unmarshal(raw, &entries))
	require.NotEmpty(t, entries)
	require.Equal(t, "completed", entries[0].Status)
	require.Len(t, entries[0].Answers, 2)
	require.Equal(t, "fine", entries[0].Answers[0].Answer)
	require.Equal(t, "nope", entries[0].Answers[1].Answer)
	require.NotEmpty(t, entries[0].CompletedAt)
}
