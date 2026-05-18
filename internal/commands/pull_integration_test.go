//go:build integration

package commands_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/aditya-mitra/questionnairebot/internal/bot"
	"github.com/aditya-mitra/questionnairebot/internal/commands"
	"github.com/aditya-mitra/questionnairebot/internal/handler"
	"github.com/aditya-mitra/questionnairebot/internal/loader"
	"github.com/aditya-mitra/questionnairebot/internal/session"
	"github.com/aditya-mitra/questionnairebot/internal/storage"
)

type pullSender struct {
	mu      sync.Mutex
	msgs    []string
	pickers []pullPicker
}

type pullPicker struct {
	Text    string
	Options []bot.PickerOption
}

func (p *pullSender) Send(text string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.msgs = append(p.msgs, text)
	return nil
}
func (p *pullSender) SendMarkdown(string) error { return nil }
func (p *pullSender) SendPicker(text string, opts []bot.PickerOption) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pickers = append(p.pickers, pullPicker{Text: text, Options: append([]bot.PickerOption(nil), opts...)})
	return nil
}
func (p *pullSender) AckCallback(string) error { return nil }

// TEST-06: past-due cron cycle → /pull prepends a skipped entry per missed tick
// and surfaces the next-upcoming cron in the picker.
func TestPullSkipsPastDueAndSurfacesNextUpcoming(t *testing.T) {
	tmp := t.TempDir()
	loc := time.UTC

	q := &loader.Questionnaire{
		Slug: "fivemin", Name: "FiveMin", Schedule: "*/5 * * * *", Timezone: "UTC", Location: loc,
		Questions: []loader.Question{{Question: "How was it?"}},
	}

	// Baseline 09:35, now 10:00. ApplyPastDueSkips fires for ticks strictly between
	// baseline and now (sched.Next(09:35)=09:40 .. 09:55; 10:00 is NOT strictly
	// before now), so 4 skipped entries are prepended. Next-upcoming is 10:05.
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, loc)
	baseline := now.Add(-25 * time.Minute) // 09:35
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "fivemin"), 0o755))
	require.NoError(t, storage.PrependCompleted(tmp, "fivemin", baseline, baseline, loc,
		[]storage.AnswerPair{{Question: "How was it?", Answer: "ok"}}))

	sessions := session.NewManager(tmp)
	flow := handler.New(nil, sessions, tmp, []*loader.Questionnaire{q})
	pull := commands.NewPull(flow, func() time.Time { return now })

	sender := &pullSender{}
	require.NoError(t, pull.Handle(sender))

	// Read answers.yaml; expect 5 skipped entries prepended (newest first), then completed.
	raw, err := os.ReadFile(filepath.Join(tmp, "fivemin", "answers.yaml"))
	require.NoError(t, err)
	var entries []storage.Entry
	require.NoError(t, yaml.Unmarshal(raw, &entries))

	skipped := 0
	for _, e := range entries {
		if e.Status == "skipped" {
			skipped++
		}
	}
	require.Equal(t, 4, skipped, "4 missed ticks strictly between baseline and now → 4 skipped entries (entries=%+v)", entries)
	// Final entry must still be the original completed baseline.
	require.Equal(t, "completed", entries[len(entries)-1].Status)

	// Picker shown with exactly one button pointing at the next-upcoming tick (10:05).
	require.Len(t, sender.pickers, 1, "exactly one picker call")
	require.Len(t, sender.pickers[0].Options, 1, "exactly one questionnaire surfaced")
	wantTick := time.Date(2026, 5, 18, 10, 5, 0, 0, loc).UTC().Format(time.RFC3339)
	require.Equal(t, "start:fivemin:"+wantTick, sender.pickers[0].Options[0].CallbackData,
		"callback must point to next strictly-future tick")
	require.Empty(t, sender.msgs, "must not also send a 'no pending' message")
}
