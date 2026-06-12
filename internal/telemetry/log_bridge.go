package telemetry

import (
	"io"
	"log"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

// installLogBridge routes the stdlib default logger through the OTel
// LoggerProvider (secondary goal) while KEEPING stderr output intact: every
// log.Printf line is both printed to stderr and emitted as an OTel log record.
//
// Records carry no trace_id — the std log call sites are context-less, so the
// active span cannot be attached without a wider log.Printf -> slog(ctx)
// migration. That full correlation is a documented follow-up, not a bug here.
//
// Only called from the enabled path, after global.SetLoggerProvider.
func installLogBridge(lp *sdklog.LoggerProvider) {
	logger := otelslog.NewLogger(scopeName, otelslog.WithLoggerProvider(lp))
	log.SetOutput(io.MultiWriter(os.Stderr, &otelLogWriter{logger: logger}))
	// Drop the std timestamp prefix: the OTel record stamps its own time, and
	// the stderr half stays readable without it.
	log.SetFlags(0)
}

// otelLogWriter adapts an *slog.Logger to io.Writer so the stdlib logger can fan
// out to OTel. Each Write (one log.Printf line) becomes a single Info record.
type otelLogWriter struct {
	logger *slog.Logger
}

func (w *otelLogWriter) Write(p []byte) (int, error) {
	w.logger.Info(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}
