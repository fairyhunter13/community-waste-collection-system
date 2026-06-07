package service

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

type paymentService struct {
	repo    domain.PaymentRepository
	storage domain.StorageService
}

// NewPaymentService creates a new PaymentService backed by the given repository and storage.
func NewPaymentService(repo domain.PaymentRepository, storage domain.StorageService) domain.PaymentService {
	return &paymentService{repo: repo, storage: storage}
}

func (s *paymentService) Create(ctx context.Context, req domain.CreatePaymentRequest) (*domain.Payment, error) {
	ctx, span := observability.Tracer().Start(ctx, "service.payment.Create")
	span.SetAttributes(
		attribute.String("payment.household_id", req.HouseholdID.String()),
		attribute.String("payment.waste_id", req.WasteID.String()),
		attribute.String("payment.amount", req.Amount.StringFixed(2)),
	)
	defer span.End()

	payment := &domain.Payment{
		HouseholdID: req.HouseholdID,
		WasteID:     req.WasteID,
		Amount:      req.Amount,
		Status:      domain.PaymentStatusPending,
	}
	if err := s.repo.Create(ctx, payment); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.String("payment.id", payment.ID.String()))
	span.SetStatus(codes.Ok, "")
	observability.PaymentsCreatedTotal.Inc()
	return payment, nil
}

func (s *paymentService) List(ctx context.Context, filter domain.PaymentFilter) ([]*domain.Payment, int, error) {
	ctx, span := observability.Tracer().Start(ctx, "service.payment.List")
	span.SetAttributes(attribute.Int("page", filter.Page), attribute.Int("per_page", filter.PerPage))
	if filter.Status != nil {
		span.SetAttributes(attribute.String("filter.status", string(*filter.Status)))
	}
	if filter.HouseholdID != nil {
		span.SetAttributes(attribute.String("filter.household_id", filter.HouseholdID.String()))
	}
	defer span.End()

	payments, total, err := s.repo.List(ctx, filter)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, 0, err
	}
	span.SetAttributes(attribute.Int("result.count", total))
	span.SetStatus(codes.Ok, "")
	return payments, total, nil
}

// Confirm enforces BR-06: a proof file is required to confirm a payment.
func (s *paymentService) Confirm(ctx context.Context, id uuid.UUID, file io.Reader, size int64, contentType string) (*domain.Payment, error) {
	ctx, span := observability.Tracer().Start(ctx, "service.payment.Confirm")
	span.SetAttributes(
		attribute.String("payment.id", id.String()),
		attribute.Int64("proof.size_bytes", size),
		attribute.String("proof.content_type", contentType),
	)
	defer span.End()

	if file == nil {
		err := fmt.Errorf("proof file is required: %w", domain.ErrValidation)
		span.RecordError(err)
		span.SetStatus(codes.Error, "no proof file")
		return nil, err
	}

	payment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if payment.Status != domain.PaymentStatusPending {
		err := fmt.Errorf("payment status is %s, must be pending: %w", payment.Status, domain.ErrConflict)
		span.RecordError(err)
		span.SetStatus(codes.Error, "payment not in pending status")
		return nil, err
	}

	key := fmt.Sprintf("%s/proof_%s", id, uuid.New().String())
	proofURL, err := s.storage.Upload(ctx, key, file, size, contentType)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "s3 upload failed")
		return nil, err
	}

	paidAt := time.Now().UTC()
	if err := s.repo.Confirm(ctx, id, proofURL, paidAt); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(attribute.String("proof.url", proofURL))
	span.SetStatus(codes.Ok, "")
	observability.PaymentsConfirmedTotal.Inc()
	payment.Status = domain.PaymentStatusPaid
	payment.ProofFileURL = &proofURL
	payment.PaymentDate = &paidAt
	return payment, nil
}
