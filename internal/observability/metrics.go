package observability

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// --- Existing metric ---

// OrganicCancelsTotal counts organic pickups auto-cancelled by the background worker.
var OrganicCancelsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "waste_organic_auto_cancels_total",
	Help: "Total organic waste pickups auto-cancelled by the worker.",
})

// labelType is the Prometheus label name for waste type, shared across pickup metric vectors.
const labelType = "type"

// --- Business event metrics ---

// PickupsCreatedTotal counts successful pickup creations by waste type.
var PickupsCreatedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "waste_pickups_created_total",
	Help: "Total pickup requests created, by waste type.",
}, []string{labelType})

// PickupsCompletedTotal counts completed pickups by waste type.
var PickupsCompletedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "waste_pickups_completed_total",
	Help: "Total pickups marked completed, by waste type.",
}, []string{labelType})

// PickupsCanceledTotal counts canceled pickups by waste type and cancellation reason.
var PickupsCanceledTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "waste_pickups_canceled_total",
	Help: "Total pickups canceled. reason: manual|auto.",
}, []string{labelType, "reason"})

// PaymentsCreatedTotal counts payment records created (auto + manual).
var PaymentsCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "waste_payments_created_total",
	Help: "Total payment records created.",
})

// PaymentsConfirmedTotal counts confirmed (paid) payment records.
var PaymentsConfirmedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "waste_payments_confirmed_total",
	Help: "Total payments confirmed with proof upload.",
})

// --- Database operation metrics ---

// DbQueryDurationSeconds tracks per-operation query latency.
var DbQueryDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "db_query_duration_seconds",
	Help:    "Duration of database queries in seconds.",
	Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5},
}, []string{"table", "operation"})

// DbErrorsTotal counts database errors by table and operation.
var DbErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "db_errors_total",
	Help: "Total database errors by table and operation.",
}, []string{"table", "operation"})

// --- Worker metrics ---

// WorkerCyclesTotal counts background worker execution cycles.
var WorkerCyclesTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "worker_cycles_total",
	Help: "Total organic canceler worker cycles executed.",
})

// WorkerCycleDurationSeconds tracks how long each worker cycle takes.
var WorkerCycleDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
	Name:    "worker_cycle_duration_seconds",
	Help:    "Duration of each organic canceler worker cycle.",
	Buckets: []float64{.001, .005, .01, .05, .1, .5, 1, 5},
})

// WorkerExpiredFoundTotal counts expired organic pickups found per cycle.
var WorkerExpiredFoundTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "worker_expired_found_total",
	Help: "Total expired organic pickups found by the worker.",
})

// StartMetricsServer starts a Prometheus metrics HTTP server on the given addr (e.g. ":2112").
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
