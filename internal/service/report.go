package service

import (
	"context"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

type reportService struct {
	paymentRepo domain.PaymentRepository
}

// NewReportService creates a new ReportService backed by the payment repository.
func NewReportService(paymentRepo domain.PaymentRepository) domain.ReportService {
	return &reportService{paymentRepo: paymentRepo}
}

func (s *reportService) WasteSummary(ctx context.Context) ([]domain.WasteTypeSummary, error) {
	ctx, span := observability.Tracer().Start(ctx, "service.report.WasteSummary")
	defer span.End()

	result, err := s.paymentRepo.WasteSummary(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.Int("result.types", len(result)))
	span.SetStatus(codes.Ok, "")
	return result, nil
}

func (s *reportService) PaymentSummary(ctx context.Context) (*domain.PaymentSummaryResult, error) {
	ctx, span := observability.Tracer().Start(ctx, "service.report.PaymentSummary")
	defer span.End()

	result, err := s.paymentRepo.PaymentSummary(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.String("result.total_revenue", result.TotalRevenue.StringFixed(2)))
	span.SetStatus(codes.Ok, "")
	return result, nil
}

func (s *reportService) HouseholdHistory(ctx context.Context, id uuid.UUID) (*domain.HouseholdHistoryResult, error) {
	ctx, span := observability.Tracer().Start(ctx, "service.report.HouseholdHistory")
	span.SetAttributes(attribute.String("household.id", id.String()))
	defer span.End()

	result, err := s.paymentRepo.HouseholdHistory(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(
		attribute.Int("result.pickups", len(result.Pickups)),
		attribute.Int("result.payments", len(result.Payments)),
	)
	span.SetStatus(codes.Ok, "")
	return result, nil
}
