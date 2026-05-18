package commands

import (
	"strings"
	"time"

	"github.com/aditya-mitra/questionnairebot/internal/loader"
)

type List struct {
	Questionnaires map[string]*loader.Questionnaire
	Clock          func() time.Time
}

func NewList(qs map[string]*loader.Questionnaire, clock func() time.Time) *List {
	if clock == nil {
		clock = time.Now
	}
	return &List{Questionnaires: qs, Clock: clock}
}

func (l *List) Render() string {
	slugs := sortedSlugs(l.Questionnaires)
	var b strings.Builder
	b.WriteString("📋 Questionnaires:\n")
	now := l.Clock()
	for _, slug := range slugs {
		q := l.Questionnaires[slug]
		next, _ := NextTrigger(q, now)
		nextFmt := next.In(q.Location).Format(time.RFC3339)
		b.WriteString(q.Name + " | cron=" + q.Schedule + " | tz=" + q.Timezone + " | next=" + nextFmt + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
