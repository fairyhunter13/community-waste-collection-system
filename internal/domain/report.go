package domain

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// WasteTypeSummary holds aggregated pickup counts for a single waste type.
type WasteTypeSummary struct {
	Type     WasteType      `json:"type"      db:"type"`
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"by_status"`
}

// PaymentStatusSummary holds aggregated payment counts and revenue for a single status.
type PaymentStatusSummary struct {
	Status  PaymentStatus   `json:"status"  db:"status"`
	Count   int             `json:"count"   db:"count"`
	Revenue decimal.Decimal `json:"revenue" db:"revenue"`
}

// PaymentSummaryResult holds the full payment summary report.
type PaymentSummaryResult struct {
	ByStatus     []PaymentStatusSummary `json:"by_status"`
	TotalRevenue decimal.Decimal        `json:"total_revenue"`
}

// HouseholdHistoryResult holds a household's full pickup and payment history.
type HouseholdHistoryResult struct {
	Household *Household     `json:"household"`
	Pickups   []*WastePickup `json:"pickups"`
	Payments  []*Payment     `json:"payments"`
}

// ReportService defines operations for generating aggregated reports.
type ReportService interface {
	WasteSummary(ctx context.Context) ([]WasteTypeSummary, error)
	PaymentSummary(ctx context.Context) (*PaymentSummaryResult, error)
	HouseholdHistory(ctx context.Context, id uuid.UUID) (*HouseholdHistoryResult, error)
}
