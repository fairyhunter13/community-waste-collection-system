package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// HealthCheck returns 200 when the application and database are reachable.
func (h *Handler) HealthCheck(c echo.Context) error {
	if err := h.db.PingContext(c.Request().Context()); err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"status": "unhealthy",
			"error":  "database unreachable",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
