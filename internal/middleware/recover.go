package middleware

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

// RecoverMiddleware returns an Echo middleware that recovers from panics and returns a 500.
// The panic log is enriched with trace_id, span_id, method, path, and request_id.
func RecoverMiddleware(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					ctx := c.Request().Context()
					log := observability.EnrichLogger(logger, ctx)
					log.ErrorContext(ctx, "panic recovered",
						slog.Any("panic", r),
						slog.String("method", c.Request().Method),
						slog.String("path", c.Request().URL.Path),
						slog.String("request_id", c.Response().Header().Get(echo.HeaderXRequestID)),
					)
					err = c.JSON(http.StatusInternalServerError, map[string]any{
						"success": false,
						"error": map[string]string{
							"code":    "INTERNAL_ERROR",
							"message": "internal server error",
						},
					})
				}
			}()
			return next(c)
		}
	}
}
