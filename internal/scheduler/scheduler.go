// Package scheduler drives per-questionnaire cron timers, invoking a Handler
// for each fire.
package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/aditya-mitra/questionnairebot/internal/loader"
)

// Handler is invoked on each cron tick with the slug of the due questionnaire.
type Handler func(slug string)

type entry struct {
	slug string
	cron *cron.Cron
	id   cron.EntryID
}

// Scheduler owns one cron instance per questionnaire.
type Scheduler struct {
	entries []entry
}

// New builds a Scheduler with a cron entry for each questionnaire.
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

// Start launches all cron timers and stops them when ctx is cancelled.
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

// Stop halts all cron timers.
func (s *Scheduler) Stop() {
	for _, e := range s.entries {
		e.cron.Stop()
	}
}
