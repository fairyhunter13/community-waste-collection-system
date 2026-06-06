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

// WorkerCyclesFailedTotal counts worker cycles that ended in a panic and were
// recovered. Distinct from WorkerCyclesTotal (which includes successful
// cycles) so that a single failing cycle does not poison the success rate.
var WorkerCyclesFailedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "worker_cycles_failed_total",
	Help: "Total worker cycles that recovered from a panic.",
})

// RateLimitActiveClients reports the number of unique client IPs currently
// tracked by the per-IP rate limiter. Used to size eviction thresholds and to
// alert on unbounded growth.
var RateLimitActiveClients = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "rate_limit_active_clients",
	Help: "Current number of client IPs tracked by the per-IP rate limiter.",
})

// --- DB connection pool gauges (scraped from sql.DB.Stats every 15s) ---

// DBPoolOpenConnections — total open connections (in use + idle).
var DBPoolOpenConnections = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "db_pool_open_connections",
	Help: "Total open DB connections (idle + in use).",
})

// DBPoolInUse — connections currently checked out by application code.
var DBPoolInUse = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "db_pool_in_use_connections",
	Help: "DB connections currently in use.",
})

// DBPoolIdle — connections sitting idle in the pool.
var DBPoolIdle = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "db_pool_idle_connections",
	Help: "DB connections sitting idle in the pool.",
})

// --- S3 storage metrics ---

// S3UploadDurationSeconds tracks the latency of S3 upload operations.
var S3UploadDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
	Name:    "s3_upload_duration_seconds",
	Help:    "Duration of S3 upload operations in seconds.",
	Buckets: []float64{.05, .1, .25, .5, 1, 2.5, 5},
})

// S3UploadBytesTotal counts total bytes uploaded to S3.
var S3UploadBytesTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "s3_upload_bytes_total",
	Help: "Total bytes uploaded to S3.",
})

// S3ErrorsTotal counts total S3 operation errors.
var S3ErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "s3_errors_total",
	Help: "Total S3 operation errors.",
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
