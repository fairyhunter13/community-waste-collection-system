// Package middleware provides Echo middleware components for the HTTP layer.
package middleware

import (
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
)

type ipLimiter struct {
	limiter *rate.Limiter
}

var (
	limiters sync.Map
)

// RateLimiter returns an Echo middleware that enforces per-IP token bucket rate limiting.
func RateLimiter(cfg *config.Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			v, _ := limiters.LoadOrStore(ip, &ipLimiter{
				limiter: rate.NewLimiter(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitBurst),
			})
			l := v.(*ipLimiter)
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
