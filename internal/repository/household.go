package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

type householdRepo struct {
	db *sqlx.DB
}

// NewHouseholdRepository creates a new HouseholdRepository backed by PostgreSQL.
func NewHouseholdRepository(db *sqlx.DB) domain.HouseholdRepository {
	return &householdRepo{db: db}
}

func (r *householdRepo) Create(ctx context.Context, h *domain.Household) error {
	ctx, span := observability.Tracer().Start(ctx, "repository.household.Create")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.sql.table", "households"),
	)
	defer span.End()
	start := time.Now()

	query := `INSERT INTO households (owner_name, address)
	          VALUES (:owner_name, :address) RETURNING *`
	rows, err := r.db.NamedQueryContext(ctx, query, h)
	observability.DbQueryDurationSeconds.WithLabelValues("households", "INSERT").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("households", "INSERT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logDBErr(ctx, "householdRepo.Create", err)
		return fmt.Errorf("create household: %w", domain.ErrInternalFailure)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		if err := rows.StructScan(h); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("scan household: %w", domain.ErrInternalFailure)
		}
	}
	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("rows err: %w", domain.ErrInternalFailure)
	}
	span.SetAttributes(attribute.String("household.id", h.ID.String()))
	span.SetStatus(codes.Ok, "")
	return nil
}

func (r *householdRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Household, error) {
	ctx, span := observability.Tracer().Start(ctx, "repository.household.FindByID")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "households"),
		attribute.String("household.id", id.String()),
	)
	defer span.End()
	start := time.Now()

	var h domain.Household
	err := r.db.GetContext(ctx, &h, `SELECT * FROM households WHERE id = $1`, id)
	observability.DbQueryDurationSeconds.WithLabelValues("households", "SELECT").Observe(time.Since(start).Seconds())
	if errors.Is(err, sql.ErrNoRows) {
		span.SetStatus(codes.Ok, "not found")
		return nil, fmt.Errorf("household %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("households", "SELECT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logDBErr(ctx, "householdRepo.FindByID", err)
		return nil, fmt.Errorf("find household: %w", domain.ErrInternalFailure)
	}
	span.SetStatus(codes.Ok, "")
	return &h, nil
}

func (r *householdRepo) List(ctx context.Context, page, perPage int) ([]*domain.Household, int, error) {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}

	ctx, span := observability.Tracer().Start(ctx, "repository.household.List")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "households"),
		attribute.Int("page", page),
		attribute.Int("per_page", perPage),
	)
	defer span.End()
	start := time.Now()

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM households`).Scan(&total); err != nil {
		observability.DbErrorsTotal.WithLabelValues("households", "SELECT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logDBErr(ctx, "householdRepo.List.count", err)
		return nil, 0, fmt.Errorf("count households: %w", domain.ErrInternalFailure)
	}

	households := make([]*domain.Household, 0)
	err := r.db.SelectContext(ctx, &households, `
		SELECT * FROM households
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, perPage, (page-1)*perPage)
	observability.DbQueryDurationSeconds.WithLabelValues("households", "SELECT").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("households", "SELECT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logDBErr(ctx, "householdRepo.List", err)
		return nil, 0, fmt.Errorf("list households: %w", domain.ErrInternalFailure)
	}

	span.SetAttributes(attribute.Int("result.count", len(households)))
	span.SetStatus(codes.Ok, "")
	return households, total, nil
}

func (r *householdRepo) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := observability.Tracer().Start(ctx, "repository.household.Delete")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "DELETE"),
		attribute.String("db.sql.table", "households"),
		attribute.String("household.id", id.String()),
	)
	defer span.End()
	start := time.Now()

	result, err := r.db.ExecContext(ctx, `DELETE FROM households WHERE id = $1`, id)
	observability.DbQueryDurationSeconds.WithLabelValues("households", "DELETE").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("households", "DELETE").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logDBErr(ctx, "householdRepo.Delete", err)
		return fmt.Errorf("delete household: %w", domain.ErrInternalFailure)
	}
	n, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("rows affected: %w", domain.ErrInternalFailure)
	}
	if n == 0 {
		span.SetStatus(codes.Ok, "not found")
		return fmt.Errorf("household %s: %w", id, domain.ErrNotFound)
	}
	span.SetAttributes(attribute.Int64("db.rows_affected", n))
	span.SetStatus(codes.Ok, "")
	return nil
}
