package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

// mapPickupCreateErr maps PostgreSQL constraint violations to domain errors.
func mapPickupCreateErr(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23503" {
		return fmt.Errorf("household not found: %w", domain.ErrNotFound)
	}
	return fmt.Errorf("create pickup: %w", domain.ErrInternalFailure)
}

type pickupRepo struct {
	db *sqlx.DB
}

// NewPickupRepository creates a new PickupRepository backed by PostgreSQL.
func NewPickupRepository(db *sqlx.DB) domain.PickupRepository {
	return &pickupRepo{db: db}
}

func (r *pickupRepo) Create(ctx context.Context, p *domain.WastePickup) error {
	ctx, span := observability.Tracer().Start(ctx, "repository.pickup.Create")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.sql.table", "waste_pickups"),
	)
	defer span.End()
	start := time.Now()

	query := `INSERT INTO waste_pickups (household_id, type, status, safety_check)
	          VALUES (:household_id, :type, :status, :safety_check) RETURNING *`
	rows, err := r.db.NamedQueryContext(ctx, query, p)
	observability.DbQueryDurationSeconds.WithLabelValues("waste_pickups", "INSERT").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("waste_pickups", "INSERT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return mapPickupCreateErr(err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		if err := rows.StructScan(p); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("scan pickup: %w", domain.ErrInternalFailure)
		}
	}
	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("rows err: %w", domain.ErrInternalFailure)
	}
	span.SetAttributes(attribute.String("pickup.id", p.ID.String()))
	span.SetStatus(codes.Ok, "")
	return nil
}

func (r *pickupRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.WastePickup, error) {
	ctx, span := observability.Tracer().Start(ctx, "repository.pickup.FindByID")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "waste_pickups"),
		attribute.String("pickup.id", id.String()),
	)
	defer span.End()
	start := time.Now()

	var p domain.WastePickup
	err := r.db.GetContext(ctx, &p, `SELECT * FROM waste_pickups WHERE id = $1`, id)
	observability.DbQueryDurationSeconds.WithLabelValues("waste_pickups", "SELECT").Observe(time.Since(start).Seconds())
	if errors.Is(err, sql.ErrNoRows) {
		span.SetStatus(codes.Ok, "not found")
		return nil, fmt.Errorf("pickup %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("waste_pickups", "SELECT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("find pickup: %w", domain.ErrInternalFailure)
	}
	span.SetStatus(codes.Ok, "")
	return &p, nil
}

func (r *pickupRepo) List(ctx context.Context, filter domain.PickupFilter) ([]*domain.WastePickup, int, error) {
	page := filter.Page
	perPage := filter.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}

	ctx, span := observability.Tracer().Start(ctx, "repository.pickup.List")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "waste_pickups"),
	)
	defer span.End()
	start := time.Now()

	conds := []string{}
	args := []any{}
	n := 1

	if filter.HouseholdID != nil {
		conds = append(conds, fmt.Sprintf("household_id = $%d", n))
		args = append(args, *filter.HouseholdID)
		n++
	}
	if filter.Status != nil {
		conds = append(conds, fmt.Sprintf("status = $%d", n))
		args = append(args, *filter.Status)
		n++
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM waste_pickups %s`, where)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		observability.DbErrorsTotal.WithLabelValues("waste_pickups", "SELECT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, 0, fmt.Errorf("count pickups: %w", domain.ErrInternalFailure)
	}

	args = append(args, perPage, (page-1)*perPage)
	listQuery := fmt.Sprintf(`
		SELECT * FROM waste_pickups
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, n, n+1)

	var pickups []*domain.WastePickup
	err := r.db.SelectContext(ctx, &pickups, listQuery, args...)
	observability.DbQueryDurationSeconds.WithLabelValues("waste_pickups", "SELECT").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("waste_pickups", "SELECT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, 0, fmt.Errorf("list pickups: %w", domain.ErrInternalFailure)
	}

	span.SetAttributes(attribute.Int("result.count", len(pickups)))
	span.SetStatus(codes.Ok, "")
	return pickups, total, nil
}

func (r *pickupRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.PickupStatus, txs ...*sqlx.Tx) error {
	ctx, span := observability.Tracer().Start(ctx, "repository.pickup.UpdateStatus")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "waste_pickups"),
		attribute.String("pickup.id", id.String()),
		attribute.String("pickup.status", string(status)),
	)
	defer span.End()
	start := time.Now()

	const q = `UPDATE waste_pickups SET status = $1, updated_at = NOW() WHERE id = $2`
	var err error
	if len(txs) > 0 && txs[0] != nil {
		_, err = txs[0].ExecContext(ctx, q, status, id)
	} else {
		_, err = r.db.ExecContext(ctx, q, status, id)
	}
	observability.DbQueryDurationSeconds.WithLabelValues("waste_pickups", "UPDATE").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("waste_pickups", "UPDATE").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("update pickup status: %w", domain.ErrInternalFailure)
	}
	span.SetStatus(codes.Ok, "")
	return nil
}

func (r *pickupRepo) Schedule(ctx context.Context, id uuid.UUID, date time.Time) error {
	ctx, span := observability.Tracer().Start(ctx, "repository.pickup.Schedule")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "waste_pickups"),
		attribute.String("pickup.id", id.String()),
	)
	defer span.End()
	start := time.Now()

	_, err := r.db.ExecContext(ctx,
		`UPDATE waste_pickups SET status = 'scheduled', pickup_date = $2, updated_at = NOW() WHERE id = $1`,
		id, date,
	)
	observability.DbQueryDurationSeconds.WithLabelValues("waste_pickups", "UPDATE").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("waste_pickups", "UPDATE").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("schedule pickup: %w", domain.ErrInternalFailure)
	}
	span.SetStatus(codes.Ok, "")
	return nil
}

func (r *pickupRepo) FindExpiredOrganic(ctx context.Context, before time.Time) ([]*domain.WastePickup, error) {
	ctx, span := observability.Tracer().Start(ctx, "repository.pickup.FindExpiredOrganic")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "waste_pickups"),
		attribute.String("before", before.String()),
	)
	defer span.End()
	start := time.Now()

	var rows []domain.WastePickup
	err := r.db.SelectContext(ctx, &rows,
		`SELECT * FROM waste_pickups
		 WHERE type = 'organic' AND status = 'pending' AND created_at < $1`,
		before,
	)
	observability.DbQueryDurationSeconds.WithLabelValues("waste_pickups", "SELECT").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("waste_pickups", "SELECT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("find expired organic: %w", domain.ErrInternalFailure)
	}
	result := make([]*domain.WastePickup, len(rows))
	for i := range rows {
		p := rows[i]
		result[i] = &p
	}
	span.SetAttributes(attribute.Int("result.count", len(result)))
	span.SetStatus(codes.Ok, "")
	return result, nil
}

func (r *pickupRepo) BulkCancel(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	ctx, span := observability.Tracer().Start(ctx, "repository.pickup.BulkCancel")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "waste_pickups"),
		attribute.Int("pickup.count", len(ids)),
	)
	defer span.End()
	start := time.Now()

	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = id.String()
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE waste_pickups SET status = 'canceled', updated_at = NOW() WHERE id = ANY($1::uuid[])`,
		pq.Array(strs),
	)
	observability.DbQueryDurationSeconds.WithLabelValues("waste_pickups", "UPDATE").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("waste_pickups", "UPDATE").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("bulk cancel pickups: %w", domain.ErrInternalFailure)
	}
	span.SetStatus(codes.Ok, "")
	return nil
}

func (r *pickupRepo) CancelIfCancellable(ctx context.Context, id uuid.UUID) (bool, error) {
	ctx, span := observability.Tracer().Start(ctx, "repository.pickup.CancelIfCancellable")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "waste_pickups"),
		attribute.String("pickup.id", id.String()),
	)
	defer span.End()
	start := time.Now()

	result, err := r.db.ExecContext(ctx,
		`UPDATE waste_pickups SET status='canceled', updated_at=NOW()
		 WHERE id=$1 AND status IN ('pending','scheduled')`, id,
	)
	observability.DbQueryDurationSeconds.WithLabelValues("waste_pickups", "UPDATE").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("waste_pickups", "UPDATE").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, fmt.Errorf("cancel pickup: %w", domain.ErrInternalFailure)
	}
	n, _ := result.RowsAffected()
	span.SetAttributes(attribute.Bool("canceled", n > 0))
	span.SetStatus(codes.Ok, "")
	return n > 0, nil
}

func (r *pickupRepo) HasPendingPaymentForHousehold(ctx context.Context, householdID uuid.UUID) (bool, error) {
	ctx, span := observability.Tracer().Start(ctx, "repository.pickup.HasPendingPaymentForHousehold")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "payments"),
		attribute.String("household.id", householdID.String()),
	)
	defer span.End()
	start := time.Now()

	var exists bool
	err := r.db.GetContext(ctx, &exists,
		`SELECT EXISTS(SELECT 1 FROM payments WHERE household_id = $1 AND status = 'pending')`,
		householdID,
	)
	observability.DbQueryDurationSeconds.WithLabelValues("payments", "SELECT").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("payments", "SELECT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, fmt.Errorf("check pending payment: %w", domain.ErrInternalFailure)
	}
	span.SetAttributes(attribute.Bool("has_pending", exists))
	span.SetStatus(codes.Ok, "")
	return exists, nil
}
