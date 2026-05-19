package commands

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aditya-mitra/questionnairebot/internal/bot"
	"github.com/aditya-mitra/questionnairebot/internal/handler"
	"github.com/aditya-mitra/questionnairebot/internal/storage"
)

// User-facing message strings emitted by the /pull command.
const (
	ReplyActiveSession = "⚠️ You have an active session in progress. Please finish it first."
	ReplyAllUpToDate   = "✅ All questionnaires are up to date. Nothing to answer right now."
	PickerPrompt       = "📋 Pick a questionnaire to start:"
	ReplyBadCallback   = "❌ Invalid selection."
)

// Pull handles the /pull slash-command: surface pending questionnaires for the
// user to start manually.
type Pull struct {
	Flow  *handler.QuestionFlow
	Clock func() time.Time
}

// NewPull constructs a Pull handler bound to the given flow.
func NewPull(flow *handler.QuestionFlow, clock func() time.Time) *Pull {
	if clock == nil {
		clock = time.Now
	}
	return &Pull{Flow: flow, Clock: clock}
}

// Handle processes a /pull command — applies past-due skips, then either
// starts the single pending questionnaire or sends a picker.
func (p *Pull) Handle(sender bot.Sender) error {
	// Active-session check: any session active anywhere → block /pull.
	for slug := range p.Flow.Questionnaires {
		if p.Flow.Sessions.Get(slug) != nil {
			return sender.Send(ReplyActiveSession)
		}
	}

	now := p.Clock()
	type pending struct {
		slug string
		when time.Time
	}
	var pendings []pending
	slugs := make([]string, 0, len(p.Flow.Questionnaires))
	for slug := range p.Flow.Questionnaires {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	for _, slug := range slugs {
		q := p.Flow.Questionnaires[slug]
		if _, err := ApplyPastDueSkips(p.Flow.DataDir, q, now, p.Clock); err != nil {
			return fmt.Errorf("pull: past-due skip %s: %w", slug, err)
		}
		next, err := NextTrigger(q, now)
		if err != nil {
			return fmt.Errorf("pull: next %s: %w", slug, err)
		}
		// If the latest answers.yaml entry already completed this exact next-cycle,
		// it isn't pending. Past-due skips only prepend strictly-past ticks; the
		// next-upcoming tick is in the future, so this guard is conservative.
		last, _ := storage.LastEntry(p.Flow.DataDir, slug)
		if last != nil && last.Status == "completed" {
			nextFmt := next.In(q.Location).Format(time.RFC3339)
			if last.ScheduledFor == nextFmt {
				continue
			}
		}
		pendings = append(pendings, pending{slug: slug, when: next})
	}

	if len(pendings) == 0 {
		return sender.Send(ReplyAllUpToDate)
	}
	opts := make([]bot.PickerOption, 0, len(pendings))
	for _, pnd := range pendings {
		opts = append(opts, bot.PickerOption{
			Label:        p.Flow.Questionnaires[pnd.slug].Name,
			CallbackData: "start:" + pnd.slug + ":" + pnd.when.UTC().Format(time.RFC3339),
		})
	}
	return sender.SendPicker(PickerPrompt, opts)
}

// HandleCallback processes an inline-keyboard "start:<slug>:<rfc3339>" callback.
func (p *Pull) HandleCallback(sender bot.Sender, data string) error {
	parts := strings.SplitN(data, ":", 3)
	if len(parts) != 3 || parts[0] != "start" {
		return sender.Send(ReplyBadCallback)
	}
	slug := parts[1]
	scheduled, err := time.Parse(time.RFC3339, parts[2])
	if err != nil {
		return sender.Send(ReplyBadCallback)
	}
	if _, ok := p.Flow.Questionnaires[slug]; !ok {
		return sender.Send(ReplyBadCallback)
	}
	return p.Flow.StartQuestionnaire(slug, scheduled)
}
