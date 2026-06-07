package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
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

	log := observability.FromContext(ctx).With(slog.String("op", "PaymentService.Create"))
	log.DebugContext(ctx, "begin",
		slog.String("household_id", req.HouseholdID.String()),
		slog.String("waste_id", req.WasteID.String()),
	)

	payment := &domain.Payment{
		HouseholdID: req.HouseholdID,
		WasteID:     req.WasteID,
		Amount:      req.Amount,
		Status:      domain.PaymentStatusPending,
	}
	if err := s.repo.Create(ctx, payment); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.ErrorContext(ctx, "create failed", slog.Any("err", err))
		return nil, err
	}
	span.SetAttributes(attribute.String("payment.id", payment.ID.String()))
	span.SetStatus(codes.Ok, "")
	observability.PaymentsCreatedTotal.Inc()
	log.InfoContext(ctx, "created",
		slog.String("payment_id", payment.ID.String()),
		slog.String("amount", req.Amount.StringFixed(2)),
	)
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
		observability.FromContext(ctx).ErrorContext(ctx, "list payments failed",
			slog.String("op", "PaymentService.List"),
			slog.Any("err", err),
		)
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

	log := observability.FromContext(ctx).With(slog.String("op", "PaymentService.Confirm"))
	log.DebugContext(ctx, "begin", slog.String("payment_id", id.String()))

	if file == nil {
		err := fmt.Errorf("proof file is required: %w", domain.ErrValidation)
		span.RecordError(err)
		span.SetStatus(codes.Error, "no proof file")
		log.WarnContext(ctx, "rejected: no proof file", slog.String("payment_id", id.String()))
		return nil, err
	}

	payment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.ErrorContext(ctx, "find payment failed", slog.String("payment_id", id.String()), slog.Any("err", err))
		return nil, err
	}
	if payment.Status != domain.PaymentStatusPending {
		err := fmt.Errorf("payment status is %s, must be pending: %w", payment.Status, domain.ErrConflict)
		span.RecordError(err)
		span.SetStatus(codes.Error, "payment not in pending status")
		log.WarnContext(ctx, "rejected: invalid status",
			slog.String("payment_id", id.String()),
			slog.String("status", string(payment.Status)),
		)
		return nil, err
	}

	key := fmt.Sprintf("%s/proof_%s", id, uuid.New().String())
	proofURL, err := s.storage.Upload(ctx, key, file, size, contentType)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "s3 upload failed")
		log.ErrorContext(ctx, "proof upload failed", slog.String("payment_id", id.String()), slog.Any("err", err))
		return nil, err
	}

	paidAt := time.Now().UTC()
	if err := s.repo.Confirm(ctx, id, proofURL, paidAt); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.ErrorContext(ctx, "confirm failed", slog.String("payment_id", id.String()), slog.Any("err", err))
		return nil, err
	}

	span.SetAttributes(attribute.String("proof.url", proofURL))
	span.SetStatus(codes.Ok, "")
	observability.PaymentsConfirmedTotal.Inc()
	payment.Status = domain.PaymentStatusPaid
	payment.ProofFileURL = &proofURL
	payment.PaymentDate = &paidAt
	log.InfoContext(ctx, "confirmed",
		slog.String("payment_id", id.String()),
		slog.String("household_id", payment.HouseholdID.String()),
	)
	return payment, nil
}
