// Package worker contains background goroutines that maintain system invariants.
package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

// OrganicCanceler periodically cancels organic pickups that have been pending
// longer than the configured cutoff period without being scheduled.
type OrganicCanceler struct {
	repo     domain.PickupRepository
	logger   *slog.Logger
	interval time.Duration
	cutoff   time.Duration
}

// NewOrganicCanceler creates a new OrganicCanceler.
func NewOrganicCanceler(repo domain.PickupRepository, logger *slog.Logger, cfg *config.Config) *OrganicCanceler {
	return &OrganicCanceler{
		repo:     repo,
		logger:   logger,
		interval: cfg.WorkerCancelInterval,
		cutoff:   time.Duration(cfg.WorkerOrganicCutoffDays) * 24 * time.Hour,
	}
}

// Start runs the canceler loop until ctx is cancelled.
func (w *OrganicCanceler) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.logger.Info("organic canceler started", "interval", w.interval, "cutoff", w.cutoff)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("organic canceler stopping")
			return
		case <-ticker.C:
			w.runWithRecover(ctx)
		}
	}
}

// runWithRecover wraps run with a deferred panic recover so a single failing
// cycle does not kill the goroutine and silently stop BR-04 enforcement.
func (w *OrganicCanceler) runWithRecover(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			w.logger.Error("organic canceler panic recovered", "panic", r)
			observability.WorkerCyclesFailedTotal.Inc()
		}
	}()
	w.run(ctx)
}

// run enforces BR-04: organic pickups not scheduled within the cutoff period are automatically cancelled.
func (w *OrganicCanceler) run(ctx context.Context) {
	ctx, span := observability.Tracer().Start(ctx, "worker.organicCanceler.run")
	defer span.End()
	start := time.Now()

	observability.WorkerCyclesTotal.Inc()

	cutoffTime := time.Now().UTC().Add(-w.cutoff)

	pickups, err := w.repo.FindExpiredOrganic(ctx, cutoffTime)
	if err != nil {
		w.logger.Error("find stale organic pickups", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "find expired organic failed")
		observability.WorkerCycleDurationSeconds.Observe(time.Since(start).Seconds())
		return
	}

	span.SetAttributes(attribute.Int("worker.expired_count", len(pickups)))
	observability.WorkerExpiredFoundTotal.Add(float64(len(pickups)))

	if len(pickups) == 0 {
		span.SetStatus(codes.Ok, "")
		observability.WorkerCycleDurationSeconds.Observe(time.Since(start).Seconds())
		return
	}

	ids := make([]uuid.UUID, len(pickups))
	for i, p := range pickups {
		ids[i] = p.ID
	}

	if err := w.repo.BulkCancel(ctx, ids); err != nil {
		w.logger.Error("bulk cancel organic pickups", "error", err, "count", len(ids))
		span.RecordError(err)
		span.SetStatus(codes.Error, "bulk cancel failed")
		observability.WorkerCycleDurationSeconds.Observe(time.Since(start).Seconds())
		return
	}

	span.SetAttributes(attribute.Int("worker.canceled_count", len(ids)))
	span.SetStatus(codes.Ok, "")
	observability.OrganicCancelsTotal.Add(float64(len(ids)))
	observability.PickupsCanceledTotal.WithLabelValues("organic", "auto").Add(float64(len(ids)))
	observability.WorkerCycleDurationSeconds.Observe(time.Since(start).Seconds())
	w.logger.Info("auto-cancelled stale organic pickups", "count", len(ids))
}
