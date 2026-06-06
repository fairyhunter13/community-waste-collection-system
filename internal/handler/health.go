package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

const (
	healthFieldStatus = "status"
	healthFieldChecks = "checks"
)

// HealthCheck (liveness) — returns 200 whenever the process is bound and serving.
// It is intentionally cheap and does NOT touch the database: a flapping DB must
// not cause Kubernetes to restart pods that are otherwise alive.
func (h *Handler) HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{healthFieldStatus: "ok"})
}

// ReadyCheck (readiness) — returns 200 only when the dependencies needed to
// serve real traffic are available. Currently: DB reachable via PingContext.
// Used by Kubernetes / load balancers to gate routing.
func (h *Handler) ReadyCheck(c echo.Context) error {
	if h.db == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]any{
			healthFieldStatus: "unready",
			healthFieldChecks: map[string]string{"db": "not_configured"},
		})
	}
	if err := h.db.PingContext(c.Request().Context()); err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]any{
			healthFieldStatus: "unready",
			healthFieldChecks: map[string]string{"db": "unreachable"},
		})
	}
	return c.JSON(http.StatusOK, map[string]any{
		healthFieldStatus: "ready",
		healthFieldChecks: map[string]string{"db": "ok"},
	})
}
