package middleware

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

// RequestLogger returns an Echo middleware that logs each request with slog,
// enriched with trace_id and span_id from the active OTel span.
func RequestLogger(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			duration := time.Since(start)

			req := c.Request()
			res := c.Response()

			log := observability.EnrichLogger(logger, req.Context())
			attrs := []any{
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.Int("status", res.Status),
				slog.Duration("duration", duration),
				slog.String("ip", c.RealIP()),
			}
			if rid := res.Header().Get(echo.HeaderXRequestID); rid != "" {
				attrs = append(attrs, slog.String("request_id", rid))
			}

			if err != nil {
				attrs = append(attrs, slog.String("error", err.Error()))
				log.ErrorContext(req.Context(), "request", attrs...)
			} else {
				log.InfoContext(req.Context(), "request", attrs...)
			}
			return err
		}
	}
}
