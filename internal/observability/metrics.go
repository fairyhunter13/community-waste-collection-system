package observability

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// OrganicCancelsTotal counts the number of organic pickups auto-cancelled by the background worker.
var OrganicCancelsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "waste_organic_auto_cancels_total",
	Help: "Total number of organic waste pickups auto-cancelled by the worker.",
})

// StartMetricsServer starts a Prometheus metrics HTTP server on the given addr (e.g. ":2112").
// It returns a shutdown function and any startup error.
func StartMetricsServer(addr string) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(fmt.Sprintf("metrics server: %v", err))
		}
	}()

	return srv, nil
}
