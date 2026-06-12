//go:build integration

// White-box (package telemetry) so the test can drive the installProviders seam
// with in-memory exporters — a deterministic assertion without a real OTLP
// collector. Ordering matters: the disabled-path test runs first (source order),
// before the enabled-path test installs recording global providers.
package telemetry

import (
	"context"
	"sync"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/aditya-mitra/questionnairebot/internal/config"
)

// capturingExporter records exported span names. Unlike tracetest.InMemoryExporter,
// its Shutdown is a no-op, so spans flushed by the batch processor during the
// provider shutdown survive for assertion.
type capturingExporter struct {
	mu    sync.Mutex
	names []string
}

func (e *capturingExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, s := range spans {
		e.names = append(e.names, s.Name())
	}
	return nil
}

func (e *capturingExporter) Shutdown(context.Context) error { return nil }

func (e *capturingExporter) snapshot() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.names...)
}

// TestSetupDisabledIsNoop proves the opt-in gate: with no telemetry configured,
// Setup installs nothing, returns a usable no-op shutdown, and the tracer yields
// a non-recording span — i.e. byte-for-byte identical to a build without OTel.
func TestSetupDisabledIsNoop(t *testing.T) {
	shutdown, err := Setup(context.Background(), &config.Config{
		ServiceName:      "questionnairebot-test",
		TelemetryEnabled: false,
	})
	if err != nil {
		t.Fatalf("Setup (disabled): %v", err)
	}
	if shutdown == nil {
		t.Fatal("Setup returned a nil shutdown")
	}

	_, span := Tracer().Start(context.Background(), "noop")
	if span.IsRecording() {
		t.Error("span IsRecording = true on the disabled path, want false")
	}
	span.End()

	if err := shutdown(context.Background()); err != nil {
		t.Errorf("no-op shutdown returned error: %v", err)
	}
}

// TestInstallProvidersEmits drives the enabled path through the test seam: a span
// is exported (flushed on shutdown) and the questionnaire.fired counter is
// collected from a manual metric reader.
func TestInstallProvidersEmits(t *testing.T) {
	spanExp := &capturingExporter{}
	reader := sdkmetric.NewManualReader()

	shutdown := installProviders(resource.Default(), spanExp, reader, nil)

	ctx, span := Tracer().Start(context.Background(), "questionnaire.fire")
	if !span.IsRecording() {
		t.Error("span IsRecording = false on the enabled path, want true")
	}
	RecordFired(ctx, "daily")
	span.End()

	// Collect metrics before shutting the MeterProvider down.
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("metric collect: %v", err)
	}
	if !hasMetric(rm, "questionnaire.fired") {
		t.Errorf("metric questionnaire.fired not emitted; got %v", metricNames(rm))
	}

	// Shutdown flushes the batch span processor, exporting the span.
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	names := spanExp.snapshot()
	found := false
	for _, n := range names {
		if n == "questionnaire.fire" {
			found = true
		}
	}
	if !found {
		t.Errorf("span questionnaire.fire not exported; got %v", names)
	}
}

func hasMetric(rm metricdata.ResourceMetrics, name string) bool {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return true
			}
		}
	}
	return false
}

func metricNames(rm metricdata.ResourceMetrics) []string {
	var names []string
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			names = append(names, m.Name)
		}
	}
	return names
}
