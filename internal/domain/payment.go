package domain

import (
	"context"
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

// PaymentRepository defines data access operations for payments.
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

// PaymentService defines business operations for payments.
type PaymentService interface {
	Create(ctx context.Context, req CreatePaymentRequest) (*Payment, error)
	List(ctx context.Context, filter PaymentFilter) ([]*Payment, int, error)
	Confirm(ctx context.Context, id uuid.UUID, file io.Reader, size int64, contentType string) (*Payment, error)
}
