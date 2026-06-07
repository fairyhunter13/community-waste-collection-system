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
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

// mapPaymentCreateErr maps PostgreSQL constraint violations to domain errors.
func mapPaymentCreateErr(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code {
		case "23503": // foreign_key_violation
			return fmt.Errorf("referenced household or waste pickup not found: %w", domain.ErrNotFound)
		case "23505": // unique_violation
			return fmt.Errorf("payment for this pickup already exists: %w", domain.ErrConflict)
		case "23514": // check_violation
			return fmt.Errorf("amount must be positive: %w", domain.ErrValidation)
		}
	}
	return fmt.Errorf("create payment: %w", domain.ErrInternalFailure)
}

type paymentRepo struct {
	db *sqlx.DB
}

// NewPaymentRepository creates a new PaymentRepository backed by PostgreSQL.
func NewPaymentRepository(db *sqlx.DB) domain.PaymentRepository {
	return &paymentRepo{db: db}
}

func (r *paymentRepo) Create(ctx context.Context, p *domain.Payment) error {
	ctx, span := observability.Tracer().Start(ctx, "repository.payment.Create")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.sql.table", "payments"),
	)
	defer span.End()
	start := time.Now()

	query := `INSERT INTO payments (household_id, waste_id, amount, status)
	          VALUES (:household_id, :waste_id, :amount, :status) RETURNING *`
	rows, err := r.db.NamedQueryContext(ctx, query, p)
	observability.DbQueryDurationSeconds.WithLabelValues("payments", "INSERT").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("payments", "INSERT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return mapPaymentCreateErr(err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		if err := rows.StructScan(p); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("scan payment: %w", domain.ErrInternalFailure)
		}
	}
	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("rows err: %w", domain.ErrInternalFailure)
	}
	span.SetAttributes(attribute.String("payment.id", p.ID.String()))
	span.SetStatus(codes.Ok, "")
	return nil
}

func (r *paymentRepo) CreateWithTx(ctx context.Context, tx *sqlx.Tx, p *domain.Payment) error {
	ctx, span := observability.Tracer().Start(ctx, "repository.payment.CreateWithTx")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.sql.table", "payments"),
		attribute.Bool("db.in_transaction", true),
	)
	defer span.End()
	start := time.Now()

	query := `INSERT INTO payments (household_id, waste_id, amount, status)
	          VALUES (:household_id, :waste_id, :amount, :status) RETURNING *`
	rows, err := tx.NamedQuery(query, p)
	observability.DbQueryDurationSeconds.WithLabelValues("payments", "INSERT").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("payments", "INSERT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return mapPaymentCreateErr(err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		if err := rows.StructScan(p); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("scan payment (tx): %w", domain.ErrInternalFailure)
		}
	}
	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("rows err: %w", domain.ErrInternalFailure)
	}
	span.SetAttributes(attribute.String("payment.id", p.ID.String()))
	span.SetStatus(codes.Ok, "")
	return nil
}

func (r *paymentRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	ctx, span := observability.Tracer().Start(ctx, "repository.payment.FindByID")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "payments"),
		attribute.String("payment.id", id.String()),
	)
	defer span.End()
	start := time.Now()

	var p domain.Payment
	err := r.db.GetContext(ctx, &p, `SELECT * FROM payments WHERE id = $1`, id)
	observability.DbQueryDurationSeconds.WithLabelValues("payments", "SELECT").Observe(time.Since(start).Seconds())
	if errors.Is(err, sql.ErrNoRows) {
		span.SetStatus(codes.Ok, "not found")
		return nil, fmt.Errorf("payment %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("payments", "SELECT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("find payment: %w", domain.ErrInternalFailure)
	}
	span.SetStatus(codes.Ok, "")
	return &p, nil
}

func (r *paymentRepo) List(ctx context.Context, filter domain.PaymentFilter) ([]*domain.Payment, int, error) {
	page := filter.Page
	perPage := filter.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}

	ctx, span := observability.Tracer().Start(ctx, "repository.payment.List")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "payments"),
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
	if filter.DateFrom != nil {
		conds = append(conds, fmt.Sprintf("payment_date >= $%d", n))
		args = append(args, *filter.DateFrom)
		n++
	}
	if filter.DateTo != nil {
		conds = append(conds, fmt.Sprintf("payment_date <= $%d", n))
		args = append(args, *filter.DateTo)
		n++
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, perPage, (page-1)*perPage)

	query := fmt.Sprintf(`
		SELECT *, COUNT(*) OVER() AS total_count
		FROM payments
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, n, n+1)

	type paymentRow struct {
		domain.Payment
		TotalCount int `db:"total_count"`
	}
	var rows []paymentRow
	err := r.db.SelectContext(ctx, &rows, query, args...)
	observability.DbQueryDurationSeconds.WithLabelValues("payments", "SELECT").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("payments", "SELECT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, 0, fmt.Errorf("list payments: %w", domain.ErrInternalFailure)
	}

	payments := make([]*domain.Payment, len(rows))
	total := 0
	for i, row := range rows {
		p := row.Payment
		payments[i] = &p
		total = row.TotalCount
	}
	span.SetAttributes(attribute.Int("result.count", total))
	span.SetStatus(codes.Ok, "")
	return payments, total, nil
}

func (r *paymentRepo) Confirm(ctx context.Context, id uuid.UUID, proofURL string, paidAt time.Time) error {
	ctx, span := observability.Tracer().Start(ctx, "repository.payment.Confirm")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "payments"),
		attribute.String("payment.id", id.String()),
	)
	defer span.End()
	start := time.Now()

	result, err := r.db.ExecContext(ctx,
		`UPDATE payments SET status='paid', proof_file_url=$2, payment_date=$3, updated_at=NOW()
		 WHERE id=$1 AND status='pending'`,
		id, proofURL, paidAt,
	)
	observability.DbQueryDurationSeconds.WithLabelValues("payments", "UPDATE").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("payments", "UPDATE").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("confirm payment: %w", domain.ErrInternalFailure)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		span.SetStatus(codes.Ok, "already confirmed or not found")
		return fmt.Errorf("payment already confirmed or not found: %w", domain.ErrConflict)
	}
	span.SetAttributes(attribute.Int64("db.rows_affected", n))
	span.SetStatus(codes.Ok, "")
	return nil
}

func (r *paymentRepo) WasteSummary(ctx context.Context) ([]domain.WasteTypeSummary, error) {
	ctx, span := observability.Tracer().Start(ctx, "repository.payment.WasteSummary")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "waste_pickups"),
	)
	defer span.End()
	start := time.Now()

	type summaryRow struct {
		Type   domain.WasteType    `db:"type"`
		Status domain.PickupStatus `db:"status"`
		Count  int                 `db:"count"`
	}
	var rows []summaryRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT type, status, COUNT(*) AS count
		FROM waste_pickups
		GROUP BY type, status
		ORDER BY type, status
	`)
	observability.DbQueryDurationSeconds.WithLabelValues("waste_pickups", "SELECT").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("waste_pickups", "SELECT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("waste summary: %w", domain.ErrInternalFailure)
	}

	index := make(map[domain.WasteType]*domain.WasteTypeSummary)
	for _, row := range rows {
		s, ok := index[row.Type]
		if !ok {
			s = &domain.WasteTypeSummary{
				Type:     row.Type,
				ByStatus: make(map[string]int),
			}
			index[row.Type] = s
		}
		s.Total += row.Count
		s.ByStatus[string(row.Status)] = row.Count
	}

	result := make([]domain.WasteTypeSummary, 0, len(index))
	for _, s := range index {
		result = append(result, *s)
	}
	span.SetAttributes(attribute.Int("result.types", len(result)))
	span.SetStatus(codes.Ok, "")
	return result, nil
}

func (r *paymentRepo) PaymentSummary(ctx context.Context) (*domain.PaymentSummaryResult, error) {
	ctx, span := observability.Tracer().Start(ctx, "repository.payment.PaymentSummary")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "payments"),
	)
	defer span.End()
	start := time.Now()

	type summaryRow struct {
		Status  domain.PaymentStatus `db:"status"`
		Count   int                  `db:"count"`
		Revenue decimal.Decimal      `db:"revenue"`
	}
	var rows []summaryRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT status, COUNT(*) AS count,
		       COALESCE(SUM(amount), 0) AS revenue
		FROM payments
		GROUP BY status
	`)
	observability.DbQueryDurationSeconds.WithLabelValues("payments", "SELECT").Observe(time.Since(start).Seconds())
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("payments", "SELECT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("payment summary: %w", domain.ErrInternalFailure)
	}

	statuses := make([]domain.PaymentStatusSummary, len(rows))
	totalRevenue := decimal.Zero
	for i, row := range rows {
		statuses[i] = domain.PaymentStatusSummary{
			Status:  row.Status,
			Count:   row.Count,
			Revenue: row.Revenue,
		}
		if row.Status == domain.PaymentStatusPaid {
			totalRevenue = row.Revenue
		}
	}
	span.SetAttributes(attribute.String("total_revenue", totalRevenue.StringFixed(2)))
	span.SetStatus(codes.Ok, "")
	return &domain.PaymentSummaryResult{
		ByStatus:     statuses,
		TotalRevenue: totalRevenue,
	}, nil
}

func (r *paymentRepo) HouseholdHistory(ctx context.Context, householdID uuid.UUID) (*domain.HouseholdHistoryResult, error) {
	ctx, span := observability.Tracer().Start(ctx, "repository.payment.HouseholdHistory")
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("household.id", householdID.String()),
	)
	defer span.End()
	start := time.Now()

	var household domain.Household
	err := r.db.GetContext(ctx, &household, `SELECT * FROM households WHERE id = $1`, householdID)
	observability.DbQueryDurationSeconds.WithLabelValues("households", "SELECT").Observe(time.Since(start).Seconds())
	if errors.Is(err, sql.ErrNoRows) {
		span.SetStatus(codes.Ok, "not found")
		return nil, fmt.Errorf("household %s: %w", householdID, domain.ErrNotFound)
	}
	if err != nil {
		observability.DbErrorsTotal.WithLabelValues("households", "SELECT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("household history: %w", domain.ErrInternalFailure)
	}

	var pickupRows []domain.WastePickup
	t2 := time.Now()
	if err := r.db.SelectContext(ctx, &pickupRows,
		`SELECT * FROM waste_pickups WHERE household_id = $1 ORDER BY created_at DESC`,
		householdID,
	); err != nil {
		observability.DbQueryDurationSeconds.WithLabelValues("waste_pickups", "SELECT").Observe(time.Since(t2).Seconds())
		observability.DbErrorsTotal.WithLabelValues("waste_pickups", "SELECT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("household history pickups: %w", domain.ErrInternalFailure)
	}
	observability.DbQueryDurationSeconds.WithLabelValues("waste_pickups", "SELECT").Observe(time.Since(t2).Seconds())

	pickups := make([]*domain.WastePickup, len(pickupRows))
	for i := range pickupRows {
		p := pickupRows[i]
		pickups[i] = &p
	}

	var paymentRows []domain.Payment
	t3 := time.Now()
	if err := r.db.SelectContext(ctx, &paymentRows,
		`SELECT * FROM payments WHERE household_id = $1 ORDER BY created_at DESC`,
		householdID,
	); err != nil {
		observability.DbQueryDurationSeconds.WithLabelValues("payments", "SELECT").Observe(time.Since(t3).Seconds())
		observability.DbErrorsTotal.WithLabelValues("payments", "SELECT").Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("household history payments: %w", domain.ErrInternalFailure)
	}
	observability.DbQueryDurationSeconds.WithLabelValues("payments", "SELECT").Observe(time.Since(t3).Seconds())

	payments := make([]*domain.Payment, len(paymentRows))
	for i := range paymentRows {
		p := paymentRows[i]
		payments[i] = &p
	}

	span.SetAttributes(
		attribute.Int("result.pickups", len(pickups)),
		attribute.Int("result.payments", len(payments)),
	)
	span.SetStatus(codes.Ok, "")
	return &domain.HouseholdHistoryResult{
		Household: &household,
		Pickups:   pickups,
		Payments:  payments,
	}, nil
}
