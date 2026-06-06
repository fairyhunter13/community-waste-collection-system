package middleware

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/trace"
)

// RequestLogger returns an Echo middleware that logs each request with slog.
func RequestLogger(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			duration := time.Since(start)

			req := c.Request()
			res := c.Response()

			attrs := []any{
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.Int("status", res.Status),
				slog.Duration("duration", duration),
				slog.String("ip", c.RealIP()),
			}
			if span := trace.SpanFromContext(req.Context()); span.SpanContext().IsValid() {
				attrs = append(attrs,
					slog.String("trace_id", span.SpanContext().TraceID().String()),
					slog.String("span_id", span.SpanContext().SpanID().String()),
				)
			}
			if rid := res.Header().Get(echo.HeaderXRequestID); rid != "" {
				attrs = append(attrs, slog.String("request_id", rid))
			}

			if err != nil {
				attrs = append(attrs, slog.String("error", err.Error()))
				logger.ErrorContext(req.Context(), "request", attrs...)
			} else {
				logger.InfoContext(req.Context(), "request", attrs...)
			}
			return err
		}
	}
}
