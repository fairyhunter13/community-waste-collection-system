package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
)

type reportService struct {
	paymentRepo domain.PaymentRepository
}

// NewReportService creates a new ReportService backed by the payment repository.
func NewReportService(paymentRepo domain.PaymentRepository) domain.ReportService {
	return &reportService{paymentRepo: paymentRepo}
}

func (s *reportService) WasteSummary(ctx context.Context) ([]domain.WasteTypeSummary, error) {
	return s.paymentRepo.WasteSummary(ctx)
}

func (s *reportService) PaymentSummary(ctx context.Context) (*domain.PaymentSummaryResult, error) {
	return s.paymentRepo.PaymentSummary(ctx)
}

func (s *reportService) HouseholdHistory(ctx context.Context, id uuid.UUID) (*domain.HouseholdHistoryResult, error) {
	return s.paymentRepo.HouseholdHistory(ctx, id)
}
