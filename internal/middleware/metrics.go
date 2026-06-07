package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests by method, path, and status code.",
	}, []string{"method", "path", "status"})

	httpRequestDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5},
	}, []string{"method", "path"})
)

// RequestMetrics records Prometheus HTTP metrics for every request.
func RequestMetrics() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			duration := time.Since(start).Seconds()

			// Use the matched route pattern, not the raw URL, to avoid label cardinality explosion.
			path := c.Path()
			if path == "" {
				path = "unknown"
			}
			method := c.Request().Method
			status := strconv.Itoa(c.Response().Status)
			if c.Response().Status == 0 {
				status = strconv.Itoa(http.StatusOK)
			}

			httpRequestsTotal.WithLabelValues(method, path, status).Inc()
			httpRequestDurationSeconds.WithLabelValues(method, path).Observe(duration)
			return err
		}
	}
}
