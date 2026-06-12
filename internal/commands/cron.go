// Package commands implements cron triggers, /pull, /status, /list.
package commands

import (
	"context"
	"log"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/aditya-mitra/questionnairebot/internal/bot"
	"github.com/aditya-mitra/questionnairebot/internal/handler"
	"github.com/aditya-mitra/questionnairebot/internal/storage"
	"github.com/aditya-mitra/questionnairebot/internal/telemetry"
)

// PickerSender is satisfied by *bot.Bot and by test recorders.
type PickerSender = bot.Sender

// PickerOption is re-exported from bot to give command code a stable name.
type PickerOption = bot.PickerOption

// CronBus debounces cron fire events and routes them to either an immediate
// session-start (single questionnaire due) or a picker prompt (multiple due
// within the debounce window).
type CronBus struct {
	Flow         *handler.QuestionFlow
	PickerSender PickerSender
	Clock        func() time.Time
	Window       time.Duration

	fires chan fireEvent
}

type fireEvent struct {
	slug string
	when time.Time
}

// NewCronBus constructs a CronBus with a 1-second debounce window.
func NewCronBus(flow *handler.QuestionFlow, picker PickerSender, clock func() time.Time) *CronBus {
	if clock == nil {
		clock = time.Now
	}
	return &CronBus{
		Flow:         flow,
		PickerSender: picker,
		Clock:        clock,
		Window:       1 * time.Second,
		fires:        make(chan fireEvent, 16),
	}
}

// Fire enqueues a cron tick for the given slug. Non-blocking; drops the event
// (with a log line) if the buffer is full.
func (b *CronBus) Fire(slug string, when time.Time) {
	select {
	case b.fires <- fireEvent{slug: slug, when: when}:
	default:
		log.Printf("commands: cron fire buffer full, dropping %s", slug)
	}
}

// Run blocks on fire events until ctx is cancelled, flushing the debounce
// buffer after each window expires.
func (b *CronBus) Run(ctx context.Context) {
	var pending []fireEvent
	var timer *time.Timer
	var timerC <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-b.fires:
			pending = append(pending, ev)
			if timer == nil {
				timer = time.NewTimer(b.Window)
				timerC = timer.C
			}
		case <-timerC:
			b.flush(ctx, pending)
			pending = nil
			timer = nil
			timerC = nil
		}
	}
}

func (b *CronBus) flush(ctx context.Context, events []fireEvent) {
	filtered := make([]fireEvent, 0, len(events))
	for _, ev := range events {
		q, ok := b.Flow.Questionnaires[ev.slug]
		if !ok {
			continue
		}
		if b.Flow.Sessions.Get(ev.slug) != nil {
			continue
		}
		last, err := storage.LastEntry(ctx, b.Flow.DataDir, ev.slug)
		if err == nil && last != nil && last.Status == "completed" {
			whenFmt := ev.when.In(q.Location).Format(time.RFC3339)
			if last.ScheduledFor == whenFmt {
				continue
			}
		}
		filtered = append(filtered, ev)
	}

	switch len(filtered) {
	case 0:
		return
	case 1:
		ev := filtered[0]
		ctx, span := telemetry.Tracer().Start(ctx, "questionnaire.fire",
			trace.WithAttributes(attribute.String("questionnaire.slug", ev.slug)))
		telemetry.RecordFired(ctx, ev.slug)
		if err := b.Flow.StartQuestionnaire(ctx, ev.slug, ev.when); err != nil {
			telemetry.RecordError(ctx, "fire")
			log.Printf("commands: start questionnaire %s: %v", ev.slug, err)
		}
		span.End()
	default:
		opts := make([]PickerOption, 0, len(filtered))
		for _, ev := range filtered {
			q := b.Flow.Questionnaires[ev.slug]
			data := "start:" + ev.slug + ":" + time.Time(ev.when).UTC().Format(time.RFC3339)
			opts = append(opts, PickerOption{Label: q.Name, CallbackData: data})
		}
		if err := b.PickerSender.SendPicker("📋 Multiple questionnaires are due. Which would you like to start?", opts); err != nil {
			log.Printf("commands: send picker: %v", err)
		}
	}
}
