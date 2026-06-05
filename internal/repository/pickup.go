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

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
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
	query := `INSERT INTO waste_pickups (household_id, type, status, safety_check)
	          VALUES (:household_id, :type, :status, :safety_check) RETURNING *`
	rows, err := r.db.NamedQueryContext(ctx, query, p)
	if err != nil {
		return mapPickupCreateErr(err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		if err := rows.StructScan(p); err != nil {
			return fmt.Errorf("scan pickup: %w", domain.ErrInternalFailure)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows err: %w", domain.ErrInternalFailure)
	}
	return nil
}

func (r *pickupRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.WastePickup, error) {
	var p domain.WastePickup
	err := r.db.GetContext(ctx, &p, `SELECT * FROM waste_pickups WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("pickup %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("find pickup: %w", domain.ErrInternalFailure)
	}
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
	args = append(args, perPage, (page-1)*perPage)

	query := fmt.Sprintf(`
		SELECT *, COUNT(*) OVER() AS total_count
		FROM waste_pickups
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, n, n+1)

	type pickupRow struct {
		domain.WastePickup
		TotalCount int `db:"total_count"`
	}
	var rows []pickupRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list pickups: %w", domain.ErrInternalFailure)
	}

	pickups := make([]*domain.WastePickup, len(rows))
	total := 0
	for i, row := range rows {
		p := row.WastePickup
		pickups[i] = &p
		total = row.TotalCount
	}
	return pickups, total, nil
}

func (r *pickupRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.PickupStatus, txs ...*sqlx.Tx) error {
	const q = `UPDATE waste_pickups SET status = $1, updated_at = NOW() WHERE id = $2`
	var err error
	if len(txs) > 0 && txs[0] != nil {
		_, err = txs[0].ExecContext(ctx, q, status, id)
	} else {
		_, err = r.db.ExecContext(ctx, q, status, id)
	}
	if err != nil {
		return fmt.Errorf("update pickup status: %w", domain.ErrInternalFailure)
	}
	return nil
}

func (r *pickupRepo) Schedule(ctx context.Context, id uuid.UUID, date time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE waste_pickups SET status = 'scheduled', pickup_date = $2, updated_at = NOW() WHERE id = $1`,
		id, date,
	)
	if err != nil {
		return fmt.Errorf("schedule pickup: %w", domain.ErrInternalFailure)
	}
	return nil
}

func (r *pickupRepo) FindExpiredOrganic(ctx context.Context, before time.Time) ([]*domain.WastePickup, error) {
	var rows []domain.WastePickup
	err := r.db.SelectContext(ctx, &rows,
		`SELECT * FROM waste_pickups
		 WHERE type = 'organic' AND status = 'pending' AND created_at < $1`,
		before,
	)
	if err != nil {
		return nil, fmt.Errorf("find expired organic: %w", domain.ErrInternalFailure)
	}
	result := make([]*domain.WastePickup, len(rows))
	for i := range rows {
		p := rows[i]
		result[i] = &p
	}
	return result, nil
}

func (r *pickupRepo) BulkCancel(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = id.String()
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE waste_pickups SET status = 'canceled', updated_at = NOW() WHERE id = ANY($1::uuid[])`,
		pq.Array(strs),
	)
	if err != nil {
		return fmt.Errorf("bulk cancel pickups: %w", domain.ErrInternalFailure)
	}
	return nil
}

func (r *pickupRepo) CancelIfCancellable(ctx context.Context, id uuid.UUID) (bool, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE waste_pickups SET status='canceled', updated_at=NOW()
		 WHERE id=$1 AND status IN ('pending','scheduled')`, id,
	)
	if err != nil {
		return false, fmt.Errorf("cancel pickup: %w", domain.ErrInternalFailure)
	}
	n, _ := result.RowsAffected()
	return n > 0, nil
}

func (r *pickupRepo) HasPendingPaymentForHousehold(ctx context.Context, householdID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists,
		`SELECT EXISTS(SELECT 1 FROM payments WHERE household_id = $1 AND status = 'pending')`,
		householdID,
	)
	if err != nil {
		return false, fmt.Errorf("check pending payment: %w", domain.ErrInternalFailure)
	}
	return exists, nil
}
