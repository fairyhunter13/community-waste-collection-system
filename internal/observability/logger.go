// Package observability provides structured logging, distributed tracing, and metrics.
package observability

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
)

// NewLogger creates a slog.Logger configured from the application config.
func NewLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level, AddSource: true}

	var handler slog.Handler
	if cfg.LogFormat == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// EnrichLogger returns l with trace_id and span_id attrs derived from the OTel
// span on ctx. Returns l unchanged when ctx carries no valid span.
func EnrichLogger(l *slog.Logger, ctx context.Context) *slog.Logger {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return l
	}
	return l.With(
		slog.String("trace_id", sc.TraceID().String()),
		slog.String("span_id", sc.SpanID().String()),
	)
}

// FromContext returns slog.Default() enriched with trace_id and span_id from
// the OTel span on ctx. Use at service / repository / worker layer where no
// logger is injected explicitly.
func FromContext(ctx context.Context) *slog.Logger {
	return EnrichLogger(slog.Default(), ctx)
}
