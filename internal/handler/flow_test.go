package handler

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/aditya-mitra/questionnairebot/internal/bot"
	"github.com/aditya-mitra/questionnairebot/internal/loader"
	"github.com/aditya-mitra/questionnairebot/internal/session"
	"github.com/aditya-mitra/questionnairebot/internal/storage"
)

type recordingSender struct {
	mu       sync.Mutex
	msgs     []string
	markdown []string
	pickers  []pickerCall
	acks     []string
}

type pickerCall struct {
	Text    string
	Options []bot.PickerOption
}

func (r *recordingSender) Send(text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, text)
	return nil
}

func (r *recordingSender) SendMarkdown(text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.markdown = append(r.markdown, text)
	return nil
}

func (r *recordingSender) SendPicker(text string, options []bot.PickerOption) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pickers = append(r.pickers, pickerCall{Text: text, Options: append([]bot.PickerOption(nil), options...)})
	return nil
}

func (r *recordingSender) AckCallback(callbackID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.acks = append(r.acks, callbackID)
	return nil
}

func TestQuestionFlowFullCycle(t *testing.T) {
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
	sessions := session.NewManager(tmp)
	sender := &recordingSender{}
	now := time.Date(2026, 5, 18, 9, 0, 0, 0, loc)
	flow := New(sender, sessions, tmp, []*loader.Questionnaire{q})
	flow.Now = func() time.Time { return now }

	if err := flow.StartQuestionnaire("daily", now); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(sender.msgs) != 1 || sender.msgs[0] != "Q1?" {
		t.Fatalf("Q1 send = %v / %v", sender.msgs, sender.markdown)
	}

	if err := flow.HandleAnswer("daily", "A1"); err != nil {
		t.Fatalf("HandleAnswer 1: %v", err)
	}
	if len(sender.markdown) != 1 || !strings.Contains(sender.markdown[0], "_Example: Ex2_") {
		t.Fatalf("Q2 markdown = %v", sender.markdown)
	}

	if err := flow.HandleAnswer("daily", "A2"); err != nil {
		t.Fatalf("HandleAnswer 2: %v", err)
	}
	if len(sender.msgs) != 2 || sender.msgs[1] != "Q3?" {
		t.Fatalf("Q3 send = %v", sender.msgs)
	}

	if err := flow.HandleAnswer("daily", "A3"); err != nil {
		t.Fatalf("HandleAnswer 3: %v", err)
	}
	if len(sender.msgs) != 3 || !strings.Contains(sender.msgs[2], "✅ Daily complete!") {
		t.Fatalf("completion send = %v", sender.msgs)
	}

	if _, err := os.Stat(filepath.Join(tmp, "daily", "session.yaml")); !os.IsNotExist(err) {
		t.Errorf("session.yaml should be gone, stat err = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(tmp, "daily", "answers.yaml"))
	if err != nil {
		t.Fatalf("read answers: %v", err)
	}
	var entries []storage.Entry
	if err := yaml.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries) != 1 || entries[0].Status != "completed" || len(entries[0].Answers) != 3 {
		t.Fatalf("entries = %+v", entries)
	}
	wantAnswers := []string{"A1", "A2", "A3"}
	for i, want := range wantAnswers {
		if entries[0].Answers[i].Answer != want {
			t.Errorf("answers[%d] = %q, want %q", i, entries[0].Answers[i].Answer, want)
		}
	}
}

func TestFinalizeIfDoneOrphan(t *testing.T) {
	tmp := t.TempDir()
	loc := time.UTC
	q := &loader.Questionnaire{
		Slug: "x", Name: "X", Location: loc,
		Questions: []loader.Question{{Question: "Q1?"}},
	}
	sessions := session.NewManager(tmp)
	sender := &recordingSender{}
	flow := New(sender, sessions, tmp, []*loader.Questionnaire{q})
	flow.Now = func() time.Time { return time.Date(2026, 5, 18, 0, 0, 0, 0, loc) }

	t0 := flow.Now()
	if _, err := sessions.Start("x", t0, t0, loc); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := sessions.RecordAnswer("x", "Q1?", "A1"); err != nil {
		t.Fatalf("RecordAnswer: %v", err)
	}
	// Now CurrentQuestionIndex == 1 == len(questions). Crash-resume scenario.
	done, err := flow.FinalizeIfDone("x")
	if err != nil || !done {
		t.Fatalf("FinalizeIfDone = (%v, %v)", done, err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "x", "session.yaml")); !os.IsNotExist(err) {
		t.Errorf("session.yaml should be gone")
	}
	if len(sender.msgs) != 1 || !strings.Contains(sender.msgs[0], "✅ X complete!") {
		t.Errorf("completion msg = %v", sender.msgs)
	}
}
