package repository

import (
	"context"
	"log/slog"

	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

// logDBErr logs err at Error level with the named repository operation.
// Call this immediately before returning an unexpected DB error so that the
// log line carries the trace_id and span_id from ctx for Loki correlation.
// Do NOT call for errors that are expected domain conditions (ErrNotFound,
// ErrConflict) — those are logged at Info level by the service layer.
func logDBErr(ctx context.Context, op string, err error) {
	observability.FromContext(ctx).ErrorContext(ctx, "db error",
		slog.String("op", op),
		slog.Any("err", err),
	)
}
