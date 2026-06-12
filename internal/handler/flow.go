// Package handler implements the question/answer state machine and Telegram
// dispatcher. QuestionFlow is decoupled from telegram-bot-api via the Sender
// interface so it can be tested without a real bot.
package handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/aditya-mitra/questionnairebot/internal/bot"
	"github.com/aditya-mitra/questionnairebot/internal/loader"
	"github.com/aditya-mitra/questionnairebot/internal/session"
	"github.com/aditya-mitra/questionnairebot/internal/storage"
	"github.com/aditya-mitra/questionnairebot/internal/telemetry"
)

// slugAttr is the span attribute every flow span carries.
func slugAttr(slug string) trace.SpanStartEventOption {
	return trace.WithAttributes(attribute.String("questionnaire.slug", slug))
}

// Sender is an alias for bot.Sender — kept here as a stable name for code
// that already uses handler.Sender.
type Sender = bot.Sender

// QuestionFlow drives the question/answer state machine for one user, backed
// by an in-memory session manager and on-disk answer storage.
type QuestionFlow struct {
	Sender         Sender
	Sessions       *session.Manager
	DataDir        string
	Questionnaires map[string]*loader.Questionnaire
	Now            func() time.Time
}

// New constructs a QuestionFlow over the given questionnaires.
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

// StartQuestionnaire opens a new session for slug and sends the first question.
func (f *QuestionFlow) StartQuestionnaire(ctx context.Context, slug string, scheduled time.Time) error {
	ctx, span := telemetry.Tracer().Start(ctx, "flow.startQuestionnaire", slugAttr(slug))
	defer span.End()
	q, ok := f.Questionnaires[slug]
	if !ok {
		return fmt.Errorf("handler: unknown questionnaire %q", slug)
	}
	if _, err := f.Sessions.Start(slug, scheduled, f.Now(), q.Location); err != nil {
		return fmt.Errorf("handler: start session: %w", err)
	}
	return f.SendQuestion(ctx, slug, 0)
}

// SendQuestion delivers the idx-th question of slug to the user.
func (f *QuestionFlow) SendQuestion(ctx context.Context, slug string, idx int) error {
	_, span := telemetry.Tracer().Start(ctx, "flow.sendQuestion", slugAttr(slug))
	defer span.End()
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

// HandleAnswer records the user's reply to the current question and either
// sends the next question or finalises the session.
func (f *QuestionFlow) HandleAnswer(ctx context.Context, slug, text string, messageID int) error {
	ctx, span := telemetry.Tracer().Start(ctx, "flow.handleAnswer", slugAttr(slug))
	defer span.End()
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
	if err := f.Sessions.RecordAnswer(slug, current.Question, text, messageID); err != nil {
		return err
	}
	telemetry.RecordAnswer(ctx, slug)
	s = f.Sessions.Get(slug)
	if s.CurrentQuestionIndex < len(q.Questions) {
		return f.SendQuestion(ctx, slug, s.CurrentQuestionIndex)
	}
	return f.finalize(ctx, slug, s, q)
}

// FinalizeIfDone finalises the slug session if all questions are answered.
// Returns whether finalisation ran.
func (f *QuestionFlow) FinalizeIfDone(ctx context.Context, slug string) (bool, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "flow.finalizeIfDone", slugAttr(slug))
	defer span.End()
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
	return true, f.finalize(ctx, slug, s, q)
}

// Reply copy for edit reflection. Tests assert on the stable substrings
// "Updated" (success) and "match" (miss), not the exact bytes.
const (
	editUpdatedMsg = "✏️ Updated your answer."
	editMissMsg    = "❓ Couldn't match that edit to a saved answer — no changes made."
)

// HandleEditedAnswer reflects an edit of the user's previously-sent answer
// (matched by Telegram message id) back into the stored answer. It searches, in
// order: every active session, then every questionnaire's answers.yaml. Failing
// an id match it applies the LOCKED fallback — rewrite the most-recent answer
// ONLY when it is legacy data (message_id == 0) — and otherwise tells the user
// the edit couldn't be matched.
func (f *QuestionFlow) HandleEditedAnswer(ctx context.Context, messageID int, newText string) error {
	ctx, span := telemetry.Tracer().Start(ctx, "flow.handleEditedAnswer")
	defer span.End()
	// 1. An active session owns the answer.
	var active []string
	for slug := range f.Questionnaires {
		if f.Sessions.Get(slug) == nil {
			continue
		}
		active = append(active, slug)
		matched, err := f.Sessions.UpdateAnswerByMessageID(slug, messageID, newText)
		if err != nil {
			return err
		}
		if matched {
			return f.Sender.Send(editUpdatedMsg)
		}
	}
	// 2. A completed answers.yaml entry owns the answer.
	for slug := range f.Questionnaires {
		matched, err := storage.UpdateAnswerByMessageID(ctx, f.DataDir, slug, messageID, newText)
		if err != nil {
			return err
		}
		if matched {
			return f.Sender.Send(editUpdatedMsg)
		}
	}
	// 3. LOCKED fallback. Rewrite the most-recent answer ONLY if it is legacy
	//    (message_id == 0); the UpdateLastAnswer helpers enforce that gate.
	if len(active) == 1 {
		matched, err := f.Sessions.UpdateLastAnswer(active[0], newText)
		if err != nil {
			return err
		}
		if matched {
			return f.Sender.Send(editUpdatedMsg)
		}
	} else if slug, err := f.newestCompletedSlug(ctx); err != nil {
		return err
	} else if slug != "" {
		matched, err := storage.UpdateLastAnswer(ctx, f.DataDir, slug, newText)
		if err != nil {
			return err
		}
		if matched {
			return f.Sender.Send(editUpdatedMsg)
		}
	}
	// 4. No id match and the fallback gate blocked any rewrite.
	return f.Sender.Send(editMissMsg)
}

// newestCompletedSlug returns the slug whose newest answers.yaml entry is the
// most recent — comparing CompletedAt, falling back to ScheduledFor, parsed as
// RFC3339. Returns "" when no questionnaire has a stored entry. A timestamp that
// fails to parse is skipped rather than aborting the edit.
func (f *QuestionFlow) newestCompletedSlug(ctx context.Context) (string, error) {
	var newest string
	var newestAt time.Time
	for slug := range f.Questionnaires {
		entry, err := storage.LastEntry(ctx, f.DataDir, slug)
		if err != nil {
			return "", err
		}
		if entry == nil {
			continue
		}
		ts := entry.CompletedAt
		if ts == "" {
			ts = entry.ScheduledFor
		}
		at, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			continue
		}
		if newest == "" || at.After(newestAt) {
			newest = slug
			newestAt = at
		}
	}
	return newest, nil
}

func (f *QuestionFlow) finalize(ctx context.Context, slug string, s *session.Session, q *loader.Questionnaire) error {
	ctx, span := telemetry.Tracer().Start(ctx, "flow.finalize", slugAttr(slug))
	defer span.End()
	scheduled, err := time.Parse(time.RFC3339, s.ScheduledFor)
	if err != nil {
		return fmt.Errorf("handler: parse scheduled_for: %w", err)
	}
	pairs := make([]storage.AnswerPair, len(s.Answers))
	for i, a := range s.Answers {
		pairs[i] = storage.AnswerPair(a)
	}
	if err := storage.PrependCompleted(ctx, f.DataDir, slug, scheduled, f.Now(), q.Location, pairs); err != nil {
		return fmt.Errorf("handler: prepend completed: %w", err)
	}
	if err := f.Sessions.Delete(slug); err != nil {
		return fmt.Errorf("handler: delete session: %w", err)
	}
	return f.Sender.Send("✅ " + q.Name + " complete! Answers saved.")
}
