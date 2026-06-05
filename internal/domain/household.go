package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Household represents a registered household in the waste collection system.
type Household struct {
	ID        uuid.UUID `db:"id"         json:"id"`
	OwnerName string    `db:"owner_name" json:"owner_name"`
	Address   string    `db:"address"    json:"address"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// CreateHouseholdRequest is the input for creating a new household.
type CreateHouseholdRequest struct {
	OwnerName string `json:"owner_name" validate:"required,min=1"`
	Address   string `json:"address"    validate:"required,min=1"`
}

// HouseholdRepository defines data access operations for households.
type HouseholdRepository interface {
	Create(ctx context.Context, h *Household) error
	FindByID(ctx context.Context, id uuid.UUID) (*Household, error)
	List(ctx context.Context, page, perPage int) ([]*Household, int, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// HouseholdService defines business operations for households.
type HouseholdService interface {
	Create(ctx context.Context, req CreateHouseholdRequest) (*Household, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Household, error)
	List(ctx context.Context, page, perPage int) ([]*Household, int, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
