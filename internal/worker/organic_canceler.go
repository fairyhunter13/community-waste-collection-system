// Package worker contains background goroutines that maintain system invariants.
package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

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
			w.run(ctx)
		}
	}
}

func (w *OrganicCanceler) run(ctx context.Context) {
	cutoffTime := time.Now().UTC().Add(-w.cutoff)

	pickups, err := w.repo.FindExpiredOrganic(ctx, cutoffTime)
	if err != nil {
		w.logger.Error("find stale organic pickups", "error", err)
		return
	}

	if len(pickups) == 0 {
		return
	}

	ids := make([]uuid.UUID, len(pickups))
	for i, p := range pickups {
		ids[i] = p.ID
	}

	if err := w.repo.BulkCancel(ctx, ids); err != nil {
		w.logger.Error("bulk cancel organic pickups", "error", err, "count", len(ids))
		return
	}

	observability.OrganicCancelsTotal.Add(float64(len(ids)))
	w.logger.Info("auto-cancelled stale organic pickups", "count", len(ids))
}
