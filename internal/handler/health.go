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
// serve real traffic are available: DB reachable and object storage bucket
// accessible. Used by Kubernetes / load balancers to gate routing.
func (h *Handler) ReadyCheck(c echo.Context) error {
	checks := map[string]string{}
	ready := true

	if h.db == nil {
		checks["db"] = "not_configured"
		ready = false
	} else if err := h.db.PingContext(c.Request().Context()); err != nil {
		checks["db"] = "unreachable"
		ready = false
	} else {
		checks["db"] = "ok"
	}

	if h.storage == nil {
		checks["storage"] = "not_configured"
		ready = false
	} else if err := h.storage.Ping(c.Request().Context()); err != nil {
		checks["storage"] = "unreachable"
		ready = false
	} else {
		checks["storage"] = "ok"
	}

	if !ready {
		return c.JSON(http.StatusServiceUnavailable, map[string]any{
			healthFieldStatus: "unready",
			healthFieldChecks: checks,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{
		healthFieldStatus: "ready",
		healthFieldChecks: checks,
	})
}
