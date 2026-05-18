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
)

// TEST-04: Two pending questionnaires → /pull → picker shown → user starts one →
// completes → second still in next /pull.
//
// Note: a bot account cannot tap inline-keyboard buttons via the bot API
// (callback_query updates only fire for user-account taps). This test
// validates the picker MESSAGE STRUCTURE (text + button labels visible in the
// reply markup) for both /pull calls, but invokes the actual start via the
// cron bus rather than a synthesized button tap.
func TestE2EPullPickerWithTwoPending(t *testing.T) {
	token, chatID := requireTestEnv(t)

	dir := t.TempDir()
	writeQ := func(slug, name string) {
		d := filepath.Join(dir, slug)
		require.NoError(t, os.MkdirAll(d, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(d, "questionnaire.yaml"), []byte(""+
			"name: "+name+"\n"+
			"schedule: 0 9 * * *\n"+
			"timezone: UTC\n"+
			"questions:\n"+
			"  - question: "+name+" question?\n"), 0o644))
	}
	writeQ("qa", "QA")
	writeQ("qb", "QB")

	rig, teardown := newBotUnderTest(t, dir, token, chatID)
	defer teardown()

	probe := newProbeClient(t, rig)

	// First /pull: picker should list both QA and QB.
	probe.send(t, "/pull")
	picker := probe.waitForMessage(t, 15*time.Second, func(m tgbotapi.Message) bool {
		return m.ReplyMarkup != nil && len(m.ReplyMarkup.InlineKeyboard) >= 2
	})
	require.NotNil(t, picker.ReplyMarkup, "first /pull must include an inline keyboard")
	labels := extractLabels(picker.ReplyMarkup)
	require.Contains(t, labels, "QA")
	require.Contains(t, labels, "QB")

	// Start qa via the cron bus (the only path the bot account can drive).
	rig.bus.Fire("qa", time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC))
	probe.waitForMessage(t, 15*time.Second, func(m tgbotapi.Message) bool {
		return strings.Contains(m.Text, "QA question?")
	})
	probe.send(t, "answer-qa")
	probe.waitForMessage(t, 15*time.Second, func(m tgbotapi.Message) bool {
		return strings.Contains(m.Text, "QA complete!")
	})

	// Second /pull: picker should now offer only QB.
	probe.send(t, "/pull")
	picker2 := probe.waitForMessage(t, 15*time.Second, func(m tgbotapi.Message) bool {
		return m.ReplyMarkup != nil && len(m.ReplyMarkup.InlineKeyboard) >= 1
	})
	labels2 := extractLabels(picker2.ReplyMarkup)
	require.Contains(t, labels2, "QB", "QB must still be pending after QA completes")
	require.NotContains(t, labels2, "QA", "QA must no longer be pending")
}

func extractLabels(kb *tgbotapi.InlineKeyboardMarkup) []string {
	var out []string
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			out = append(out, btn.Text)
		}
	}
	return out
}
