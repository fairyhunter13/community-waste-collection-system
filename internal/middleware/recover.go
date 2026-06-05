package middleware

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
)

// RecoverMiddleware returns an Echo middleware that recovers from panics and returns a 500.
func RecoverMiddleware(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					logger.ErrorContext(c.Request().Context(), "panic recovered",
						slog.Any("panic", r),
						slog.String("path", c.Request().URL.Path),
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
