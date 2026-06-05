package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
)

type householdRepo struct {
	db *sqlx.DB
}

// NewHouseholdRepository creates a new HouseholdRepository backed by PostgreSQL.
func NewHouseholdRepository(db *sqlx.DB) domain.HouseholdRepository {
	return &householdRepo{db: db}
}

func (r *householdRepo) Create(ctx context.Context, h *domain.Household) error {
	query := `INSERT INTO households (owner_name, address)
	          VALUES (:owner_name, :address) RETURNING *`
	rows, err := r.db.NamedQueryContext(ctx, query, h)
	if err != nil {
		return fmt.Errorf("create household: %w", domain.ErrInternalFailure)
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.StructScan(h); err != nil {
			return fmt.Errorf("scan household: %w", domain.ErrInternalFailure)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows err: %w", domain.ErrInternalFailure)
	}
	return nil
}

func (r *householdRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Household, error) {
	var h domain.Household
	err := r.db.GetContext(ctx, &h, `SELECT * FROM households WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("household %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("find household: %w", domain.ErrInternalFailure)
	}
	return &h, nil
}

func (r *householdRepo) List(ctx context.Context, page, perPage int) ([]*domain.Household, int, error) {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}

	type householdRow struct {
		domain.Household
		TotalCount int `db:"total_count"`
	}

	var rows []householdRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT *, COUNT(*) OVER() AS total_count
		FROM households
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("list households: %w", domain.ErrInternalFailure)
	}

	households := make([]*domain.Household, len(rows))
	total := 0
	for i, row := range rows {
		h := row.Household
		households[i] = &h
		total = row.TotalCount
	}
	return households, total, nil
}

func (r *householdRepo) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM households WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete household: %w", domain.ErrInternalFailure)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", domain.ErrInternalFailure)
	}
	if n == 0 {
		return fmt.Errorf("household %s: %w", id, domain.ErrNotFound)
	}
	return nil
}
