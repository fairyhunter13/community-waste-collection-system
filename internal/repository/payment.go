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

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
)

type paymentRepo struct {
	db *sqlx.DB
}

// NewPaymentRepository creates a new PaymentRepository backed by PostgreSQL.
func NewPaymentRepository(db *sqlx.DB) domain.PaymentRepository {
	return &paymentRepo{db: db}
}

func (r *paymentRepo) Create(ctx context.Context, p *domain.Payment) error {
	query := `INSERT INTO payments (household_id, waste_id, amount, status)
	          VALUES (:household_id, :waste_id, :amount, :status) RETURNING *`
	rows, err := r.db.NamedQueryContext(ctx, query, p)
	if err != nil {
		return fmt.Errorf("create payment: %w", domain.ErrInternalFailure)
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.StructScan(p); err != nil {
			return fmt.Errorf("scan payment: %w", domain.ErrInternalFailure)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows err: %w", domain.ErrInternalFailure)
	}
	return nil
}

func (r *paymentRepo) CreateWithTx(ctx context.Context, tx *sqlx.Tx, p *domain.Payment) error {
	query := `INSERT INTO payments (household_id, waste_id, amount, status)
	          VALUES (:household_id, :waste_id, :amount, :status) RETURNING *`
	rows, err := tx.NamedQuery(query, p)
	if err != nil {
		return fmt.Errorf("create payment (tx): %w", domain.ErrInternalFailure)
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.StructScan(p); err != nil {
			return fmt.Errorf("scan payment (tx): %w", domain.ErrInternalFailure)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows err: %w", domain.ErrInternalFailure)
	}
	return nil
}

func (r *paymentRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	var p domain.Payment
	err := r.db.GetContext(ctx, &p, `SELECT * FROM payments WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("payment %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("find payment: %w", domain.ErrInternalFailure)
	}
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
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list payments: %w", domain.ErrInternalFailure)
	}

	payments := make([]*domain.Payment, len(rows))
	total := 0
	for i, row := range rows {
		p := row.Payment
		payments[i] = &p
		total = row.TotalCount
	}
	return payments, total, nil
}

func (r *paymentRepo) Confirm(ctx context.Context, id uuid.UUID, proofURL string, paidAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE payments SET status = 'paid', proof_file_url = $2, payment_date = $3, updated_at = NOW() WHERE id = $1`,
		id, proofURL, paidAt,
	)
	if err != nil {
		return fmt.Errorf("confirm payment: %w", domain.ErrInternalFailure)
	}
	return nil
}

func (r *paymentRepo) WasteSummary(ctx context.Context) ([]domain.WasteTypeSummary, error) {
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
	if err != nil {
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
	return result, nil
}

func (r *paymentRepo) PaymentSummary(ctx context.Context) (*domain.PaymentSummaryResult, error) {
	type summaryRow struct {
		Status  domain.PaymentStatus `db:"status"`
		Count   int                  `db:"count"`
		Revenue string               `db:"revenue"`
	}
	var rows []summaryRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT status, COUNT(*) AS count,
		       COALESCE(SUM(amount), 0)::text AS revenue
		FROM payments
		GROUP BY status
	`)
	if err != nil {
		return nil, fmt.Errorf("payment summary: %w", domain.ErrInternalFailure)
	}

	statuses := make([]domain.PaymentStatusSummary, len(rows))
	totalRevenue := "0"
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
	return &domain.PaymentSummaryResult{
		ByStatus:     statuses,
		TotalRevenue: totalRevenue,
	}, nil
}

func (r *paymentRepo) HouseholdHistory(ctx context.Context, householdID uuid.UUID) (*domain.HouseholdHistoryResult, error) {
	var household domain.Household
	err := r.db.GetContext(ctx, &household, `SELECT * FROM households WHERE id = $1`, householdID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("household %s: %w", householdID, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("household history: %w", domain.ErrInternalFailure)
	}

	var pickupRows []domain.WastePickup
	if err := r.db.SelectContext(ctx, &pickupRows,
		`SELECT * FROM waste_pickups WHERE household_id = $1 ORDER BY created_at DESC`,
		householdID,
	); err != nil {
		return nil, fmt.Errorf("household history pickups: %w", domain.ErrInternalFailure)
	}
	pickups := make([]*domain.WastePickup, len(pickupRows))
	for i := range pickupRows {
		p := pickupRows[i]
		pickups[i] = &p
	}

	var paymentRows []domain.Payment
	if err := r.db.SelectContext(ctx, &paymentRows,
		`SELECT * FROM payments WHERE household_id = $1 ORDER BY created_at DESC`,
		householdID,
	); err != nil {
		return nil, fmt.Errorf("household history payments: %w", domain.ErrInternalFailure)
	}
	payments := make([]*domain.Payment, len(paymentRows))
	for i := range paymentRows {
		p := paymentRows[i]
		payments[i] = &p
	}

	return &domain.HouseholdHistoryResult{
		Household: &household,
		Pickups:   pickups,
		Payments:  payments,
	}, nil
}
