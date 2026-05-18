package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/aditya-mitra/questionnairebot/internal/loader"
)

type Handler func(slug string)

type entry struct {
	slug string
	cron *cron.Cron
	id   cron.EntryID
}

type Scheduler struct {
	entries []entry
}

func New(qs []*loader.Questionnaire, h Handler) (*Scheduler, error) {
	s := &Scheduler{}
	for _, q := range qs {
		q := q
		c := cron.New(cron.WithLocation(q.Location))
		id, err := c.AddFunc(q.Schedule, func() { h(q.Slug) })
		if err != nil {
			return nil, fmt.Errorf("scheduler: %s: %w", q.Slug, err)
		}
		s.entries = append(s.entries, entry{slug: q.Slug, cron: c, id: id})
	}
	return s, nil
}

func (s *Scheduler) Start(ctx context.Context) {
	for _, e := range s.entries {
		e.cron.Start()
		next := e.cron.Entry(e.id).Next
		log.Printf("Next trigger for %s: %s", e.slug, next.Format(time.RFC3339))
	}
	go func() {
		<-ctx.Done()
		s.Stop()
	}()
}

func (s *Scheduler) Stop() {
	for _, e := range s.entries {
		e.cron.Stop()
	}
}
