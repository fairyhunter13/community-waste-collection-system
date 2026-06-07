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
	interval time.Duration
	cutoff   time.Duration
}

// NewOrganicCanceler creates a new OrganicCanceler. The logger parameter is
// accepted for backwards-compatibility but is not stored; logging is done via
// observability.FromContext so every cycle log carries its OTel trace_id.
func NewOrganicCanceler(repo domain.PickupRepository, _ *slog.Logger, cfg *config.Config) *OrganicCanceler {
	return &OrganicCanceler{
		repo:     repo,
		interval: cfg.WorkerCancelInterval,
		cutoff:   time.Duration(cfg.WorkerOrganicCutoffDays) * 24 * time.Hour,
	}
}

// Start runs the canceler loop until ctx is cancelled.
func (w *OrganicCanceler) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	observability.FromContext(ctx).InfoContext(ctx, "organic canceler started",
		slog.String("worker", "organic_canceler"),
		slog.String("interval", w.interval.String()),
		slog.String("cutoff", w.cutoff.String()),
	)

	for {
		select {
		case <-ctx.Done():
			observability.FromContext(ctx).InfoContext(ctx, "organic canceler stopping",
				slog.String("worker", "organic_canceler"),
			)
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
			observability.FromContext(ctx).ErrorContext(ctx, "organic canceler panic recovered",
				slog.String("worker", "organic_canceler"),
				slog.Any("panic", r),
			)
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

	// Each cycle gets a unique id for log correlation independent of trace_id.
	cycleID := uuid.NewString()
	log := observability.FromContext(ctx).With(
		slog.String("worker", "organic_canceler"),
		slog.String("cycle_id", cycleID),
	)
	log.DebugContext(ctx, "cycle begin", slog.Time("cutoff_before", time.Now().UTC().Add(-w.cutoff)))

	cutoffTime := time.Now().UTC().Add(-w.cutoff)

	pickups, err := w.repo.FindExpiredOrganic(ctx, cutoffTime)
	if err != nil {
		log.ErrorContext(ctx, "find stale organic pickups failed", slog.Any("err", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "find expired organic failed")
		observability.WorkerCycleDurationSeconds.Observe(time.Since(start).Seconds())
		return
	}

	span.SetAttributes(attribute.Int("worker.expired_count", len(pickups)))
	observability.WorkerExpiredFoundTotal.Add(float64(len(pickups)))

	if len(pickups) == 0 {
		log.DebugContext(ctx, "cycle complete", slog.Int("rows_canceled", 0))
		span.SetStatus(codes.Ok, "")
		observability.WorkerCycleDurationSeconds.Observe(time.Since(start).Seconds())
		return
	}

	ids := make([]uuid.UUID, len(pickups))
	for i, p := range pickups {
		ids[i] = p.ID
	}

	if err := w.repo.BulkCancel(ctx, ids); err != nil {
		log.ErrorContext(ctx, "bulk cancel organic pickups failed", slog.Any("err", err), slog.Int("count", len(ids)))
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
	log.InfoContext(ctx, "cycle complete", slog.Int("rows_canceled", len(ids)))
}
