// Package middleware provides Echo middleware components for the HTTP layer.
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	limiters       sync.Map
	evictOnce      sync.Once
	evictInterval  = 5 * time.Minute
	evictThreshold = 30 * time.Minute
)

// RateLimiter returns an Echo middleware that enforces per-IP token bucket
// rate limiting. On first invocation it launches a single background sweeper
// that evicts IPs idle for longer than evictThreshold so the underlying
// sync.Map does not grow unbounded under sustained traffic from changing IPs.
func RateLimiter(cfg *config.Config) echo.MiddlewareFunc {
	evictOnce.Do(func() { go evictIdleClients() })

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			now := time.Now()
			v, loaded := limiters.LoadOrStore(ip, &ipLimiter{
				limiter:  rate.NewLimiter(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitBurst),
				lastSeen: now,
			})
			l := v.(*ipLimiter)
			l.lastSeen = now
			if !loaded {
				observability.RateLimitActiveClients.Inc()
			}
			if !l.limiter.Allow() {
				return c.JSON(http.StatusTooManyRequests, map[string]any{
					"success": false,
					"error": map[string]string{
						"code":    "RATE_LIMITED",
						"message": "too many requests",
					},
				})
			}
			return next(c)
		}
	}
}

// evictIdleClients sweeps the per-IP limiter map every evictInterval, removing
// entries with lastSeen older than evictThreshold. Runs for the lifetime of
// the process — the cost is bounded by the active-client gauge.
func evictIdleClients() {
	ticker := time.NewTicker(evictInterval)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-evictThreshold)
		removed := 0
		limiters.Range(func(k, v any) bool {
			l := v.(*ipLimiter)
			if l.lastSeen.Before(cutoff) {
				limiters.Delete(k)
				removed++
			}
			return true
		})
		if removed > 0 {
			observability.RateLimitActiveClients.Sub(float64(removed))
		}
	}
}
