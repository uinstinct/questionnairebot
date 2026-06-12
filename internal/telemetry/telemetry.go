// Package telemetry owns the OpenTelemetry SDK lifecycle for the bot.
//
// Setup is the single entry point: it gates on whether an OTLP endpoint is
// configured (cfg.TelemetryEnabled) and, when enabled, installs real
// TracerProvider/MeterProvider/LoggerProvider + a W3C propagator and registers
// the metric instruments. When disabled it returns a no-op shutdown WITHOUT
// touching any global provider — OTel's defaults are already no-ops, so the bot
// behaves byte-for-byte identically to a build without telemetry.
//
// The package imports only internal/config; it never imports storage/handler/
// commands, so the storage/handler call sites can import it without a cycle.
package telemetry

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"

	"github.com/aditya-mitra/questionnairebot/internal/config"
)

// noopShutdown is returned whenever telemetry is disabled or setup is downgraded.
func noopShutdown(context.Context) error { return nil }

// Setup initialises the OpenTelemetry SDK from the standard OTEL_* environment
// variables and returns a shutdown function that flushes and stops every
// installed provider. When cfg.TelemetryEnabled is false it returns immediately
// with a no-op shutdown and leaves all global providers as their default no-ops.
func Setup(ctx context.Context, cfg *config.Config) (shutdown func(context.Context) error, err error) {
	if cfg == nil || !cfg.TelemetryEnabled {
		return noopShutdown, nil
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)),
	)
	if err != nil {
		return noopShutdown, fmt.Errorf("telemetry: resource: %w", err)
	}

	// autoexport.New* default to OTLP localhost:4317 even with no endpoint,
	// which is why this code path only runs once cfg.TelemetryEnabled is true.
	// IsNone* is true only when the operator explicitly set OTEL_*_EXPORTER=none.
	spanExp, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return noopShutdown, fmt.Errorf("telemetry: span exporter: %w", err)
	}
	if autoexport.IsNoneSpanExporter(spanExp) {
		spanExp = nil
	}

	metricReader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		return noopShutdown, fmt.Errorf("telemetry: metric reader: %w", err)
	}
	if autoexport.IsNoneMetricReader(metricReader) {
		metricReader = nil
	}

	logExp, err := autoexport.NewLogExporter(ctx)
	if err != nil {
		return noopShutdown, fmt.Errorf("telemetry: log exporter: %w", err)
	}
	if autoexport.IsNoneLogExporter(logExp) {
		logExp = nil
	}

	return installProviders(res, spanExp, metricReader, logExp), nil
}

// installProviders wires the (possibly-nil) exporters into SDK providers, sets
// them as the OTel globals, registers instruments, and returns an aggregated
// shutdown. A nil exporter means that signal is skipped. It is the seam the
// integration test injects in-memory exporters through.
func installProviders(res *resource.Resource, spanExp sdktrace.SpanExporter, metricReader sdkmetric.Reader, logExp sdklog.Exporter) func(context.Context) error {
	var shutdownFuncs []func(context.Context) error

	if spanExp != nil {
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithBatcher(spanExp),
		)
		otel.SetTracerProvider(tp)
		shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
	}

	if metricReader != nil {
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
			sdkmetric.WithReader(metricReader),
		)
		otel.SetMeterProvider(mp)
		shutdownFuncs = append(shutdownFuncs, mp.Shutdown)
	}

	if logExp != nil {
		lp := sdklog.NewLoggerProvider(
			sdklog.WithResource(res),
			sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		)
		global.SetLoggerProvider(lp)
		shutdownFuncs = append(shutdownFuncs, lp.Shutdown)
		installLogBridge(lp)
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Register instruments against the now-live meter so the observable gauge
	// callback fires; safe even if no MeterProvider was installed (no-op meter).
	InitInstruments()

	return func(ctx context.Context) error {
		var err error
		for i := len(shutdownFuncs) - 1; i >= 0; i-- {
			err = errors.Join(err, shutdownFuncs[i](ctx))
		}
		shutdownFuncs = nil
		return err
	}
}
