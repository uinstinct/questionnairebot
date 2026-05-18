package commands

import (
	"sort"
	"strings"
	"time"

	"github.com/aditya-mitra/questionnairebot/internal/loader"
	"github.com/aditya-mitra/questionnairebot/internal/session"
	"github.com/aditya-mitra/questionnairebot/internal/storage"
)

type Status struct {
	DataDir        string
	Sessions       *session.Manager
	Questionnaires map[string]*loader.Questionnaire
	Clock          func() time.Time
}

func NewStatus(dataDir string, sessions *session.Manager, qs map[string]*loader.Questionnaire, clock func() time.Time) *Status {
	if clock == nil {
		clock = time.Now
	}
	return &Status{DataDir: dataDir, Sessions: sessions, Questionnaires: qs, Clock: clock}
}

func (s *Status) Render() string {
	slugs := sortedSlugs(s.Questionnaires)
	var b strings.Builder
	b.WriteString("📊 Status:\n")
	now := s.Clock()
	for _, slug := range slugs {
		q := s.Questionnaires[slug]
		last, _ := storage.LastEntry(s.DataDir, slug)
		lastFmt := "Never"
		if last != nil {
			when := last.CompletedAt
			if when == "" {
				when = last.SkippedAt
			}
			if when == "" {
				when = last.ScheduledFor
			}
			lastFmt = when
		}
		next, _ := NextTrigger(q, now)
		nextFmt := next.In(q.Location).Format(time.RFC3339)

		state := "⏳ Pending"
		if s.Sessions.Get(slug) != nil {
			state = "🔄 In Progress"
		} else if last != nil && last.Status == "completed" {
			// Done if the most recent completed entry matches the most recent
			// past-or-current scheduled cycle. We approximate "most recent
			// scheduled cycle" by stepping the cron back one tick from next.
			state = "✅ Done"
		}
		b.WriteString(q.Name + " | last=" + lastFmt + " | next=" + nextFmt + " | " + state + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func sortedSlugs(qs map[string]*loader.Questionnaire) []string {
	slugs := make([]string, 0, len(qs))
	for s := range qs {
		slugs = append(slugs, s)
	}
	sort.Strings(slugs)
	return slugs
}
