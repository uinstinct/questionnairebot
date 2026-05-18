// Package handler implements the question/answer state machine and Telegram
// dispatcher. QuestionFlow is decoupled from telegram-bot-api via the Sender
// interface so it can be tested without a real bot.
package handler

import (
	"errors"
	"fmt"
	"time"

	"github.com/aditya-mitra/questionnairebot/internal/loader"
	"github.com/aditya-mitra/questionnairebot/internal/session"
	"github.com/aditya-mitra/questionnairebot/internal/storage"
)

type Sender interface {
	Send(text string) error
	SendMarkdown(text string) error
}

type QuestionFlow struct {
	Sender         Sender
	Sessions       *session.Manager
	DataDir        string
	Questionnaires map[string]*loader.Questionnaire
	Now            func() time.Time
}

func New(sender Sender, sessions *session.Manager, dataDir string, qs []*loader.Questionnaire) *QuestionFlow {
	m := make(map[string]*loader.Questionnaire, len(qs))
	for _, q := range qs {
		m[q.Slug] = q
	}
	return &QuestionFlow{
		Sender:         sender,
		Sessions:       sessions,
		DataDir:        dataDir,
		Questionnaires: m,
		Now:            time.Now,
	}
}

func (f *QuestionFlow) StartQuestionnaire(slug string, scheduled time.Time) error {
	q, ok := f.Questionnaires[slug]
	if !ok {
		return fmt.Errorf("handler: unknown questionnaire %q", slug)
	}
	if _, err := f.Sessions.Start(slug, scheduled, f.Now(), q.Location); err != nil {
		return fmt.Errorf("handler: start session: %w", err)
	}
	return f.SendQuestion(slug, 0)
}

func (f *QuestionFlow) SendQuestion(slug string, idx int) error {
	q, ok := f.Questionnaires[slug]
	if !ok {
		return fmt.Errorf("handler: unknown questionnaire %q", slug)
	}
	if idx >= len(q.Questions) {
		return nil
	}
	item := q.Questions[idx]
	if item.Example == "" {
		return f.Sender.Send(item.Question)
	}
	return f.Sender.SendMarkdown(item.Question + "\n_Example: " + item.Example + "_")
}

func (f *QuestionFlow) HandleAnswer(slug, text string) error {
	q, ok := f.Questionnaires[slug]
	if !ok {
		return fmt.Errorf("handler: unknown questionnaire %q", slug)
	}
	s := f.Sessions.Get(slug)
	if s == nil {
		return fmt.Errorf("handler: no active session for %q", slug)
	}
	if s.CurrentQuestionIndex >= len(q.Questions) {
		return errors.New("handler: session already complete")
	}
	current := q.Questions[s.CurrentQuestionIndex]
	if err := f.Sessions.RecordAnswer(slug, current.Question, text); err != nil {
		return err
	}
	s = f.Sessions.Get(slug)
	if s.CurrentQuestionIndex < len(q.Questions) {
		return f.SendQuestion(slug, s.CurrentQuestionIndex)
	}
	return f.finalize(slug, s, q)
}

func (f *QuestionFlow) FinalizeIfDone(slug string) (bool, error) {
	q, ok := f.Questionnaires[slug]
	if !ok {
		return false, fmt.Errorf("handler: unknown questionnaire %q", slug)
	}
	s := f.Sessions.Get(slug)
	if s == nil {
		return false, nil
	}
	if s.CurrentQuestionIndex < len(q.Questions) {
		return false, nil
	}
	return true, f.finalize(slug, s, q)
}

func (f *QuestionFlow) finalize(slug string, s *session.Session, q *loader.Questionnaire) error {
	scheduled, err := time.Parse(time.RFC3339, s.ScheduledFor)
	if err != nil {
		return fmt.Errorf("handler: parse scheduled_for: %w", err)
	}
	pairs := make([]storage.AnswerPair, len(s.Answers))
	for i, a := range s.Answers {
		pairs[i] = storage.AnswerPair{Question: a.Question, Answer: a.Answer}
	}
	if err := storage.PrependCompleted(f.DataDir, slug, scheduled, f.Now(), q.Location, pairs); err != nil {
		return fmt.Errorf("handler: prepend completed: %w", err)
	}
	if err := f.Sessions.Delete(slug); err != nil {
		return fmt.Errorf("handler: delete session: %w", err)
	}
	return f.Sender.Send("✅ " + q.Name + " complete! Answers saved.")
}
