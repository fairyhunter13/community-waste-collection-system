package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// WasteType represents the category of waste for a pickup request.
type WasteType string

// PickupStatus represents the lifecycle state of a waste pickup.
type PickupStatus string

// WasteType enum values.
const (
	WasteTypeOrganic    WasteType = "organic"
	WasteTypePlastic    WasteType = "plastic"
	WasteTypePaper      WasteType = "paper"
	WasteTypeElectronic WasteType = "electronic"
)

// PickupStatus enum values.
const (
	PickupStatusPending   PickupStatus = "pending"
	PickupStatusScheduled PickupStatus = "scheduled"
	PickupStatusCompleted PickupStatus = "completed"
	PickupStatusCanceled  PickupStatus = "canceled"
)

// PaymentAmounts maps waste type to the fixed payment amount in whole currency units.
var PaymentAmounts = map[WasteType]decimal.Decimal{
	WasteTypeOrganic:    decimal.RequireFromString("50000.00"),
	WasteTypePlastic:    decimal.RequireFromString("50000.00"),
	WasteTypePaper:      decimal.RequireFromString("50000.00"),
	WasteTypeElectronic: decimal.RequireFromString("100000.00"),
}

// WastePickup represents a household waste pickup request.
type WastePickup struct {
	ID          uuid.UUID    `db:"id"           json:"id"`
	HouseholdID uuid.UUID    `db:"household_id" json:"household_id"`
	Type        WasteType    `db:"type"         json:"type"`
	Status      PickupStatus `db:"status"       json:"status"`
	PickupDate  *time.Time   `db:"pickup_date"  json:"pickup_date"`
	SafetyCheck bool         `db:"safety_check" json:"safety_check"`
	CreatedAt   time.Time    `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time    `db:"updated_at"   json:"updated_at"`
}

// CreatePickupRequest is the input for creating a new pickup request.
type CreatePickupRequest struct {
	HouseholdID uuid.UUID `json:"household_id" validate:"required,db_exists_household"`
	Type        WasteType `json:"type"         validate:"required,oneof=organic plastic paper electronic"`
	SafetyCheck bool      `json:"safety_check"`
}

// SchedulePickupRequest is the input for scheduling a pickup.
type SchedulePickupRequest struct {
	PickupDate time.Time `json:"pickup_date" validate:"required"`
}

// PickupFilter defines optional filters for listing pickups.
type PickupFilter struct {
	HouseholdID *uuid.UUID
	Status      *PickupStatus
	Page        int
	PerPage     int
}

// PickupRepository persists pickup rows and enforces row-level integrity for
// the lifecycle state machine (pending → scheduled → completed/canceled).
//
// Contract:
//   - Schedule, UpdateStatus, and CancelIfCancellable use conditional UPDATEs
//     (status predicate in the WHERE clause). When the row is in the wrong
//     state, RowsAffected==0 and callers must surface [ErrConflict].
//   - Create participates in the BR-01 advisory-lock flow; callers acquire
//     pg_advisory_xact_lock keyed on household_id before invoking.
//   - HasPendingPaymentForHousehold is the BR-01 check; the partial unique
//     index on payments backs it at the DB tier.
//   - FindExpiredOrganic + BulkCancel power the BR-04 worker; both are safe to
//     call concurrently and use SKIP LOCKED semantics where required.
type PickupRepository interface {
	Create(ctx context.Context, p *WastePickup) error
	FindByID(ctx context.Context, id uuid.UUID) (*WastePickup, error)
	List(ctx context.Context, filter PickupFilter) ([]*WastePickup, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status PickupStatus, tx ...*sqlx.Tx) error
	Schedule(ctx context.Context, id uuid.UUID, date time.Time) error
	FindExpiredOrganic(ctx context.Context, before time.Time) ([]*WastePickup, error)
	BulkCancel(ctx context.Context, ids []uuid.UUID) error
	HasPendingPaymentForHousehold(ctx context.Context, householdID uuid.UUID) (bool, error)
	// CancelIfCancellable atomically cancels a pickup only if it is in a cancellable state
	// (pending or scheduled). Returns true if the row was updated, false otherwise.
	CancelIfCancellable(ctx context.Context, id uuid.UUID) (bool, error)
}

// PickupService orchestrates the pickup lifecycle and enforces BR-01..BR-05.
//
// Each method emits an OTel span; Create wraps the BR-01 advisory-lock + insert
// in a single transaction; Complete uses SELECT … FOR UPDATE on the pickup row
// inside the BR-05 atomic transaction (status update + payment insert).
// Conflicts on stale state map to [ErrConflict]; business-rule violations to
// [ErrBusinessRule]; unknown IDs to [ErrNotFound].
type PickupService interface {
	Create(ctx context.Context, req CreatePickupRequest) (*WastePickup, error)
	List(ctx context.Context, filter PickupFilter) ([]*WastePickup, int, error)
	Schedule(ctx context.Context, id uuid.UUID, req SchedulePickupRequest) (*WastePickup, error)
	Complete(ctx context.Context, id uuid.UUID) (*WastePickup, error)
	Cancel(ctx context.Context, id uuid.UUID) (*WastePickup, error)
}
