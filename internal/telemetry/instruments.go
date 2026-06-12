package telemetry

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// scopeName identifies the instrumentation scope for this module's traces,
// metrics, and logs.
const scopeName = "github.com/aditya-mitra/questionnairebot"

// Package-level instruments. They are obtained from the global meter in
// InitInstruments; until then (and on the telemetry-disabled path) they stay
// nil, so every record helper nil-guards before use.
var (
	firedCounter        metric.Int64Counter
	answersCounter      metric.Int64Counter
	errorsCounter       metric.Int64Counter
	activeSessionsGauge metric.Int64ObservableGauge //nolint:unused // held to keep the gauge registration alive

	// activeSessionsSource feeds the active-sessions observable gauge. It is read
	// from the metric callback goroutine; the supplied func (sessions.Len) is
	// itself mutex-guarded, so no extra synchronisation is needed here.
	activeSessionsSource func() int64
)

// Tracer returns the module tracer. Always safe: it yields a no-op tracer until
// Setup installs a real TracerProvider.
func Tracer() trace.Tracer {
	return otel.Tracer(scopeName)
}

// InitInstruments creates the metric instruments from the global meter and
// registers the active-sessions observable gauge. Instrument-creation errors are
// logged, never fatal — telemetry must never take the bot down.
func InitInstruments() {
	m := otel.Meter(scopeName)

	var err error
	if firedCounter, err = m.Int64Counter(
		"questionnaire.fired",
		metric.WithDescription("Number of questionnaires fired by the scheduler/cron bus"),
	); err != nil {
		log.Printf("telemetry: fired counter: %v", err)
	}
	if answersCounter, err = m.Int64Counter(
		"questionnaire.answers_recorded",
		metric.WithDescription("Number of answers recorded across all questionnaires"),
	); err != nil {
		log.Printf("telemetry: answers counter: %v", err)
	}
	if errorsCounter, err = m.Int64Counter(
		"questionnaire.errors",
		metric.WithDescription("Number of errors encountered handling updates/flows"),
	); err != nil {
		log.Printf("telemetry: errors counter: %v", err)
	}
	if activeSessionsGauge, err = m.Int64ObservableGauge(
		"questionnaire.active_sessions",
		metric.WithDescription("Number of in-progress questionnaire sessions"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			if activeSessionsSource != nil {
				o.Observe(activeSessionsSource())
			}
			return nil
		}),
	); err != nil {
		log.Printf("telemetry: active-sessions gauge: %v", err)
	}
}

// SetActiveSessionsSource registers the function the active-sessions gauge reads.
// main.go passes sessions.Len. A nil source is tolerated (the gauge observes
// nothing) until this is called.
func SetActiveSessionsSource(fn func() int64) {
	activeSessionsSource = fn
}

// RecordFired increments the questionnaires-fired counter. No-op until instruments
// are initialised (telemetry disabled).
func RecordFired(ctx context.Context, slug string) {
	if firedCounter != nil {
		firedCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("questionnaire.slug", slug)))
	}
}

// RecordAnswer increments the answers-recorded counter.
func RecordAnswer(ctx context.Context, slug string) {
	if answersCounter != nil {
		answersCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("questionnaire.slug", slug)))
	}
}

// RecordError increments the errors counter, tagged with a coarse operation kind.
func RecordError(ctx context.Context, kind string) {
	if errorsCounter != nil {
		errorsCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("error.kind", kind)))
	}
}
