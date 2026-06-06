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

// HouseholdRepository persists household records.
//
// Implementations are expected to:
//   - Generate the row's primary key inside Create when h.ID is the zero UUID.
//   - Return [ErrNotFound] (wrapped) from FindByID and Delete when the row is absent.
//   - Cascade related pickups and payments on Delete; the migration enforces this.
//   - Be safe for concurrent use; all methods take a context for cancellation.
type HouseholdRepository interface {
	Create(ctx context.Context, h *Household) error
	FindByID(ctx context.Context, id uuid.UUID) (*Household, error)
	List(ctx context.Context, page, perPage int) ([]*Household, int, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// HouseholdService is the business-rule entry point for household operations.
//
// Service methods emit an OTel span per call and translate repository sentinels
// into domain errors ([ErrNotFound], [ErrValidation]) so handlers can map them
// uniformly. Cascading Delete is the deliberate trade-off documented in
// docs/adr/0007-cascade-delete.md — callers should warn users before invoking.
type HouseholdService interface {
	Create(ctx context.Context, req CreateHouseholdRequest) (*Household, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Household, error)
	List(ctx context.Context, page, perPage int) ([]*Household, int, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
