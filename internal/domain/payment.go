package domain

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// PaymentStatus represents the lifecycle state of a payment.
type PaymentStatus string

// PaymentStatus enum values.
const (
	PaymentStatusPending PaymentStatus = "pending"
	PaymentStatusPaid    PaymentStatus = "paid"
	PaymentStatusFailed  PaymentStatus = "failed"
)

// Payment represents a payment record associated with a completed waste pickup.
type Payment struct {
	ID           uuid.UUID       `db:"id"             json:"id"`
	HouseholdID  uuid.UUID       `db:"household_id"   json:"household_id"`
	WasteID      uuid.UUID       `db:"waste_id"       json:"waste_id"`
	Amount       decimal.Decimal `db:"amount"         json:"amount"`
	PaymentDate  *time.Time      `db:"payment_date"   json:"payment_date"`
	Status       PaymentStatus   `db:"status"         json:"status"`
	ProofFileURL *string         `db:"proof_file_url" json:"proof_file_url"`
	CreatedAt    time.Time       `db:"created_at"     json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"     json:"updated_at"`
}

// MarshalJSON serializes Payment with Amount always having 2 decimal places.
func (p Payment) MarshalJSON() ([]byte, error) {
	type Alias Payment
	return json.Marshal(struct {
		Alias
		Amount string `json:"amount"`
	}{
		Alias:  Alias(p),
		Amount: p.Amount.StringFixed(2),
	})
}

// CreatePaymentRequest is the input for creating a new payment record.
type CreatePaymentRequest struct {
	HouseholdID uuid.UUID       `json:"household_id" validate:"required,db_exists_household"`
	WasteID     uuid.UUID       `json:"waste_id"     validate:"required,db_exists_pickup"`
	Amount      decimal.Decimal `json:"amount"       validate:"required,positive_decimal"`
}

// PaymentFilter defines optional filters for listing payments.
type PaymentFilter struct {
	HouseholdID *uuid.UUID
	Status      *PaymentStatus
	DateFrom    *time.Time
	DateTo      *time.Time
	Page        int
	PerPage     int
}

// PaymentRepository persists payment rows and serves report aggregations.
//
// Contract:
//   - Create and CreateWithTx insert a payment row; the unique constraint on
//     (waste_id) and the partial-unique index on (household_id) WHERE status='pending'
//     defend BR-01 / BR-06 at the DB tier. Duplicate inserts surface [ErrConflict].
//   - Confirm sets status='paid', payment_date=paidAt, proof_file_url=proofURL
//     atomically; idempotent at the SQL layer (conditional on status='pending').
//   - HouseholdHistory parallelises three queries via errgroup so wall-clock is
//     a single RTT regardless of pickup/payment row counts.
type PaymentRepository interface {
	Create(ctx context.Context, p *Payment) error
	CreateWithTx(ctx context.Context, tx *sqlx.Tx, p *Payment) error
	FindByID(ctx context.Context, id uuid.UUID) (*Payment, error)
	List(ctx context.Context, filter PaymentFilter) ([]*Payment, int, error)
	Confirm(ctx context.Context, id uuid.UUID, proofURL string, paidAt time.Time) error
	WasteSummary(ctx context.Context) ([]WasteTypeSummary, error)
	PaymentSummary(ctx context.Context) (*PaymentSummaryResult, error)
	HouseholdHistory(ctx context.Context, householdID uuid.UUID) (*HouseholdHistoryResult, error)
}

// PaymentService is the business-rule entry point for payment operations.
//
// Create produces a pending payment for a completed pickup; Confirm uploads a
// proof file to object storage and flips the row to paid. Confirm validates
// the upload's Content-Type against an allowlist (image/jpeg, image/png,
// application/pdf) and sniffs the magic bytes before persisting — the
// client-supplied Content-Type is never trusted. All methods emit OTel spans
// and map storage / repository errors into the [ErrValidation], [ErrNotFound],
// and [ErrConflict] sentinels.
type PaymentService interface {
	Create(ctx context.Context, req CreatePaymentRequest) (*Payment, error)
	List(ctx context.Context, filter PaymentFilter) ([]*Payment, int, error)
	Confirm(ctx context.Context, id uuid.UUID, file io.Reader, size int64, contentType string) (*Payment, error)
}
