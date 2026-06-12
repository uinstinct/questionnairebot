package handler

import (
	"context"
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

	if err := flow.StartQuestionnaire(context.Background(), "daily", now); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(sender.msgs) != 1 || sender.msgs[0] != "Q1?" {
		t.Fatalf("Q1 send = %v / %v", sender.msgs, sender.markdown)
	}

	if err := flow.HandleAnswer(context.Background(), "daily", "A1", 201); err != nil {
		t.Fatalf("HandleAnswer 1: %v", err)
	}
	if len(sender.markdown) != 1 || !strings.Contains(sender.markdown[0], "_Example: Ex2_") {
		t.Fatalf("Q2 markdown = %v", sender.markdown)
	}

	if err := flow.HandleAnswer(context.Background(), "daily", "A2", 202); err != nil {
		t.Fatalf("HandleAnswer 2: %v", err)
	}
	if len(sender.msgs) != 2 || sender.msgs[1] != "Q3?" {
		t.Fatalf("Q3 send = %v", sender.msgs)
	}

	if err := flow.HandleAnswer(context.Background(), "daily", "A3", 203); err != nil {
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
	wantIDs := []int{201, 202, 203}
	for i, want := range wantAnswers {
		if entries[0].Answers[i].Answer != want {
			t.Errorf("answers[%d] = %q, want %q", i, entries[0].Answers[i].Answer, want)
		}
		if entries[0].Answers[i].MessageID != wantIDs[i] {
			t.Errorf("answers[%d].MessageID = %d, want %d", i, entries[0].Answers[i].MessageID, wantIDs[i])
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
	if err := sessions.RecordAnswer("x", "Q1?", "A1", 0); err != nil {
		t.Fatalf("RecordAnswer: %v", err)
	}
	// Now CurrentQuestionIndex == 1 == len(questions). Crash-resume scenario.
	done, err := flow.FinalizeIfDone(context.Background(), "x")
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

func seedCompletedFlow(t *testing.T, dir, slug string, answers []storage.AnswerPair) {
	t.Helper()
	loc := time.UTC
	t0 := time.Date(2026, 5, 18, 9, 0, 0, 0, loc)
	if err := storage.PrependCompleted(context.Background(), dir, slug, t0, t0.Add(15*time.Minute), loc, answers); err != nil {
		t.Fatalf("PrependCompleted: %v", err)
	}
}

func readFlowEntries(t *testing.T, dir, slug string) []storage.Entry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, slug, "answers.yaml"))
	if err != nil {
		t.Fatalf("read answers.yaml: %v", err)
	}
	var entries []storage.Entry
	if err := yaml.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return entries
}

func lastSent(t *testing.T, sender *recordingSender) string {
	t.Helper()
	if len(sender.msgs) == 0 {
		t.Fatalf("no message sent")
	}
	return sender.msgs[len(sender.msgs)-1]
}

func TestHandleEditedAnswerActiveSessionMatch(t *testing.T) {
	tmp := t.TempDir()
	loc := time.UTC
	q := &loader.Questionnaire{
		Slug: "daily", Name: "Daily", Location: loc,
		Questions: []loader.Question{{Question: "Q1?"}, {Question: "Q2?"}, {Question: "Q3?"}},
	}
	sessions := session.NewManager(tmp)
	sender := &recordingSender{}
	flow := New(sender, sessions, tmp, []*loader.Questionnaire{q})

	t0 := time.Date(2026, 5, 18, 9, 0, 0, 0, loc)
	if _, err := sessions.Start("daily", t0, t0, loc); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := sessions.RecordAnswer("daily", "Q1?", "A1", 10); err != nil {
		t.Fatalf("RecordAnswer 1: %v", err)
	}
	if err := sessions.RecordAnswer("daily", "Q2?", "A2", 20); err != nil {
		t.Fatalf("RecordAnswer 2: %v", err)
	}

	if err := flow.HandleEditedAnswer(context.Background(), 20, "edited A2"); err != nil {
		t.Fatalf("HandleEditedAnswer: %v", err)
	}
	got := sessions.Get("daily")
	if got.Answers[1].Answer != "edited A2" || got.Answers[0].Answer != "A1" {
		t.Fatalf("session answers = %+v", got.Answers)
	}
	if !strings.Contains(lastSent(t, sender), "Updated") {
		t.Errorf("reply = %q, want success substring", lastSent(t, sender))
	}
}

func TestHandleEditedAnswerCompletedMatch(t *testing.T) {
	tmp := t.TempDir()
	loc := time.UTC
	q := &loader.Questionnaire{Slug: "daily", Name: "Daily", Location: loc, Questions: []loader.Question{{Question: "Q1?"}}}
	sessions := session.NewManager(tmp)
	sender := &recordingSender{}
	flow := New(sender, sessions, tmp, []*loader.Questionnaire{q})

	// Two entries; no active session. Match a sibling (not the last answer).
	seedCompletedFlow(t, tmp, "daily", []storage.AnswerPair{{Question: "Q1?", Answer: "OLD1", MessageID: 30}})
	seedCompletedFlow(t, tmp, "daily", []storage.AnswerPair{
		{Question: "Q1?", Answer: "B1", MessageID: 40},
		{Question: "Q2?", Answer: "B2", MessageID: 41},
	})

	if err := flow.HandleEditedAnswer(context.Background(), 40, "edited B1"); err != nil {
		t.Fatalf("HandleEditedAnswer: %v", err)
	}
	entries := readFlowEntries(t, tmp, "daily")
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].Answers[0].Answer != "edited B1" {
		t.Errorf("target = %q, want edited B1", entries[0].Answers[0].Answer)
	}
	if entries[0].Answers[1].Answer != "B2" {
		t.Errorf("sibling = %q, want B2 (untouched)", entries[0].Answers[1].Answer)
	}
	if entries[1].Answers[0].Answer != "OLD1" {
		t.Errorf("older entry = %q, want OLD1 (untouched)", entries[1].Answers[0].Answer)
	}
	if !strings.Contains(lastSent(t, sender), "Updated") {
		t.Errorf("reply = %q, want success substring", lastSent(t, sender))
	}
}

func TestHandleEditedAnswerLegacyFallback(t *testing.T) {
	tmp := t.TempDir()
	loc := time.UTC
	q := &loader.Questionnaire{Slug: "daily", Name: "Daily", Location: loc, Questions: []loader.Question{{Question: "Q1?"}}}
	sessions := session.NewManager(tmp)
	sender := &recordingSender{}
	flow := New(sender, sessions, tmp, []*loader.Questionnaire{q})

	// Single questionnaire, no active session, legacy last answer (no message_id).
	seedCompletedFlow(t, tmp, "daily", []storage.AnswerPair{
		{Question: "Q1?", Answer: "kept", MessageID: 50},
		{Question: "Q2?", Answer: "legacy"},
	})

	if err := flow.HandleEditedAnswer(context.Background(), 9999, "corrected"); err != nil {
		t.Fatalf("HandleEditedAnswer: %v", err)
	}
	entries := readFlowEntries(t, tmp, "daily")
	if entries[0].Answers[1].Answer != "corrected" {
		t.Errorf("legacy last answer = %q, want corrected", entries[0].Answers[1].Answer)
	}
	if entries[0].Answers[0].Answer != "kept" {
		t.Errorf("sibling = %q, want kept", entries[0].Answers[0].Answer)
	}
	if !strings.Contains(lastSent(t, sender), "Updated") {
		t.Errorf("reply = %q, want success substring", lastSent(t, sender))
	}
}

func TestHandleEditedAnswerSafetyGate(t *testing.T) {
	tmp := t.TempDir()
	loc := time.UTC
	q := &loader.Questionnaire{Slug: "daily", Name: "Daily", Location: loc, Questions: []loader.Question{{Question: "Q1?"}}}
	sessions := session.NewManager(tmp)
	sender := &recordingSender{}
	flow := New(sender, sessions, tmp, []*loader.Questionnaire{q})

	// Most-recent candidate already has a real message_id -> never clobbered.
	seedCompletedFlow(t, tmp, "daily", []storage.AnswerPair{{Question: "Q1?", Answer: "real", MessageID: 60}})

	if err := flow.HandleEditedAnswer(context.Background(), 9999, "should-not-apply"); err != nil {
		t.Fatalf("HandleEditedAnswer: %v", err)
	}
	entries := readFlowEntries(t, tmp, "daily")
	if entries[0].Answers[0].Answer != "real" {
		t.Errorf("answer clobbered: %q, want real", entries[0].Answers[0].Answer)
	}
	if !strings.Contains(lastSent(t, sender), "match") {
		t.Errorf("reply = %q, want couldn't-match substring", lastSent(t, sender))
	}
}
