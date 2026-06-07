package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

type pickupService struct {
	pickupRepo  domain.PickupRepository
	paymentRepo domain.PaymentRepository
	db          *sqlx.DB
}

// NewPickupService creates a new PickupService with access to pickup and payment repositories.
func NewPickupService(pickupRepo domain.PickupRepository, paymentRepo domain.PaymentRepository, db *sqlx.DB) domain.PickupService {
	return &pickupService{
		pickupRepo:  pickupRepo,
		paymentRepo: paymentRepo,
		db:          db,
	}
}

// Create enforces BR-01: a household with any pending payment cannot create a new pickup.
func (s *pickupService) Create(ctx context.Context, req domain.CreatePickupRequest) (*domain.WastePickup, error) {
	ctx, span := observability.Tracer().Start(ctx, "service.pickup.Create")
	span.SetAttributes(
		attribute.String("pickup.household_id", req.HouseholdID.String()),
		attribute.String("pickup.type", string(req.Type)),
	)
	defer span.End()

	log := observability.FromContext(ctx).With(slog.String("op", "PickupService.Create"))
	log.DebugContext(ctx, "begin",
		slog.String("household_id", req.HouseholdID.String()),
		slog.String("type", string(req.Type)),
	)

	hasPending, err := s.pickupRepo.HasPendingPaymentForHousehold(ctx, req.HouseholdID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.ErrorContext(ctx, "check pending payment failed", slog.Any("err", err))
		return nil, err
	}
	if hasPending {
		err := fmt.Errorf("household has a pending payment: %w", domain.ErrConflict)
		span.RecordError(err)
		span.SetStatus(codes.Error, "blocked by pending payment")
		log.WarnContext(ctx, "rejected: pending payment exists",
			slog.String("household_id", req.HouseholdID.String()),
		)
		return nil, err
	}

	pickup := &domain.WastePickup{
		HouseholdID: req.HouseholdID,
		Type:        req.Type,
		Status:      domain.PickupStatusPending,
		SafetyCheck: req.SafetyCheck,
	}
	if err := s.pickupRepo.Create(ctx, pickup); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.ErrorContext(ctx, "create failed", slog.Any("err", err))
		return nil, err
	}
	span.SetAttributes(attribute.String("pickup.id", pickup.ID.String()))
	span.SetStatus(codes.Ok, "")
	observability.PickupsCreatedTotal.WithLabelValues(string(req.Type)).Inc()
	log.InfoContext(ctx, "created",
		slog.String("pickup_id", pickup.ID.String()),
		slog.String("type", string(req.Type)),
	)
	return pickup, nil
}

func (s *pickupService) List(ctx context.Context, filter domain.PickupFilter) ([]*domain.WastePickup, int, error) {
	ctx, span := observability.Tracer().Start(ctx, "service.pickup.List")
	span.SetAttributes(attribute.Int("page", filter.Page), attribute.Int("per_page", filter.PerPage))
	if filter.Status != nil {
		span.SetAttributes(attribute.String("filter.status", string(*filter.Status)))
	}
	if filter.HouseholdID != nil {
		span.SetAttributes(attribute.String("filter.household_id", filter.HouseholdID.String()))
	}
	defer span.End()

	pickups, total, err := s.pickupRepo.List(ctx, filter)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		observability.FromContext(ctx).ErrorContext(ctx, "list pickups failed",
			slog.String("op", "PickupService.List"),
			slog.Any("err", err),
		)
		return nil, 0, err
	}
	span.SetAttributes(attribute.Int("result.count", total))
	span.SetStatus(codes.Ok, "")
	return pickups, total, nil
}

// Schedule enforces BR-02 (must be pending) and BR-03 (electronic requires safety_check=true).
func (s *pickupService) Schedule(ctx context.Context, id uuid.UUID, req domain.SchedulePickupRequest) (*domain.WastePickup, error) {
	ctx, span := observability.Tracer().Start(ctx, "service.pickup.Schedule")
	span.SetAttributes(attribute.String("pickup.id", id.String()))
	defer span.End()

	log := observability.FromContext(ctx).With(slog.String("op", "PickupService.Schedule"))
	log.DebugContext(ctx, "begin", slog.String("pickup_id", id.String()))

	pickup, err := s.pickupRepo.FindByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.ErrorContext(ctx, "find pickup failed", slog.String("pickup_id", id.String()), slog.Any("err", err))
		return nil, err
	}

	if pickup.Status != domain.PickupStatusPending {
		err := fmt.Errorf("pickup cannot be scheduled: current status is %s: %w", pickup.Status, domain.ErrConflict)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid status for scheduling")
		log.WarnContext(ctx, "rejected: invalid status",
			slog.String("pickup_id", id.String()),
			slog.String("status", string(pickup.Status)),
		)
		return nil, err
	}

	if pickup.Type == domain.WasteTypeElectronic && !pickup.SafetyCheck {
		err := fmt.Errorf("electronic pickup requires safety_check to be true before scheduling: %w", domain.ErrBusinessRule)
		span.RecordError(err)
		span.SetStatus(codes.Error, "electronic safety check required")
		log.WarnContext(ctx, "rejected: electronic safety check required",
			slog.String("pickup_id", id.String()),
		)
		return nil, err
	}

	if req.PickupDate.Before(time.Now()) {
		err := fmt.Errorf("pickup_date must be in the future: %w", domain.ErrValidation)
		span.RecordError(err)
		span.SetStatus(codes.Error, "pickup_date in the past")
		log.WarnContext(ctx, "rejected: pickup_date in the past",
			slog.String("pickup_id", id.String()),
		)
		return nil, err
	}

	if err := s.pickupRepo.Schedule(ctx, id, req.PickupDate); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.ErrorContext(ctx, "schedule failed", slog.String("pickup_id", id.String()), slog.Any("err", err))
		return nil, err
	}

	span.SetAttributes(
		attribute.String("pickup.type", string(pickup.Type)),
		attribute.String("pickup.pickup_date", req.PickupDate.String()),
	)
	span.SetStatus(codes.Ok, "")
	pickup.Status = domain.PickupStatusScheduled
	pickup.PickupDate = &req.PickupDate
	log.InfoContext(ctx, "scheduled",
		slog.String("pickup_id", id.String()),
		slog.Time("pickup_date", req.PickupDate),
	)
	return pickup, nil
}

// Complete enforces BR-05: completing a pickup atomically creates its payment record.
func (s *pickupService) Complete(ctx context.Context, id uuid.UUID) (*domain.WastePickup, error) {
	ctx, span := observability.Tracer().Start(ctx, "service.pickup.Complete")
	span.SetAttributes(attribute.String("pickup.id", id.String()))
	defer span.End()

	log := observability.FromContext(ctx).With(slog.String("op", "PickupService.Complete"))
	log.DebugContext(ctx, "begin", slog.String("pickup_id", id.String()))

	pickup, err := s.pickupRepo.FindByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.ErrorContext(ctx, "find pickup failed", slog.String("pickup_id", id.String()), slog.Any("err", err))
		return nil, err
	}
	if pickup.Status != domain.PickupStatusScheduled {
		err := fmt.Errorf("pickup status is %s, must be scheduled: %w", pickup.Status, domain.ErrConflict)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid status for completion")
		log.WarnContext(ctx, "rejected: invalid status",
			slog.String("pickup_id", id.String()),
			slog.String("status", string(pickup.Status)),
		)
		return nil, err
	}

	amount := domain.PaymentAmounts[pickup.Type]
	payment := &domain.Payment{
		HouseholdID: pickup.HouseholdID,
		WasteID:     pickup.ID,
		Amount:      amount,
		Status:      domain.PaymentStatusPending,
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		err = fmt.Errorf("begin tx: %w", domain.ErrInternalFailure)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to begin transaction")
		log.ErrorContext(ctx, "begin tx failed", slog.String("pickup_id", id.String()), slog.Any("err", err))
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = s.pickupRepo.UpdateStatus(ctx, id, domain.PickupStatusCompleted, tx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.ErrorContext(ctx, "update status failed", slog.String("pickup_id", id.String()), slog.Any("err", err))
		return nil, err
	}
	if err = s.paymentRepo.CreateWithTx(ctx, tx, payment); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.ErrorContext(ctx, "create payment failed", slog.String("pickup_id", id.String()), slog.Any("err", err))
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		err = fmt.Errorf("commit: %w", domain.ErrInternalFailure)
		span.RecordError(err)
		span.SetStatus(codes.Error, "commit failed")
		log.ErrorContext(ctx, "commit failed", slog.String("pickup_id", id.String()), slog.Any("err", err))
		return nil, err
	}

	span.SetAttributes(
		attribute.String("pickup.type", string(pickup.Type)),
		attribute.String("payment.amount", amount.StringFixed(2)),
	)
	span.SetStatus(codes.Ok, "")
	observability.PickupsCompletedTotal.WithLabelValues(string(pickup.Type)).Inc()
	observability.PaymentsCreatedTotal.Inc()
	pickup.Status = domain.PickupStatusCompleted
	log.InfoContext(ctx, "completed",
		slog.String("pickup_id", id.String()),
		slog.String("payment_id", payment.ID.String()),
		slog.String("amount", amount.StringFixed(2)),
	)
	return pickup, nil
}

// Cancel transitions a pickup to canceled status atomically using a conditional UPDATE.
func (s *pickupService) Cancel(ctx context.Context, id uuid.UUID) (*domain.WastePickup, error) {
	ctx, span := observability.Tracer().Start(ctx, "service.pickup.Cancel")
	span.SetAttributes(attribute.String("pickup.id", id.String()))
	defer span.End()

	log := observability.FromContext(ctx).With(slog.String("op", "PickupService.Cancel"))
	log.DebugContext(ctx, "begin", slog.String("pickup_id", id.String()))

	ok, err := s.pickupRepo.CancelIfCancellable(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.ErrorContext(ctx, "cancel check failed", slog.String("pickup_id", id.String()), slog.Any("err", err))
		return nil, err
	}
	if !ok {
		pickup, err := s.pickupRepo.FindByID(ctx, id)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			log.ErrorContext(ctx, "find pickup failed", slog.String("pickup_id", id.String()), slog.Any("err", err))
			return nil, err
		}
		var conflictErr error
		switch pickup.Status {
		case domain.PickupStatusCompleted:
			conflictErr = fmt.Errorf("pickup status is completed, cannot cancel: %w", domain.ErrConflict)
		default:
			conflictErr = fmt.Errorf("pickup is already canceled: %w", domain.ErrConflict)
		}
		span.RecordError(conflictErr)
		span.SetStatus(codes.Error, "cannot cancel pickup")
		log.WarnContext(ctx, "rejected: cannot cancel",
			slog.String("pickup_id", id.String()),
			slog.String("status", string(pickup.Status)),
		)
		return nil, conflictErr
	}

	pickup, err := s.pickupRepo.FindByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.ErrorContext(ctx, "find pickup after cancel failed", slog.String("pickup_id", id.String()), slog.Any("err", err))
		return nil, err
	}
	span.SetAttributes(attribute.String("pickup.type", string(pickup.Type)))
	span.SetStatus(codes.Ok, "")
	observability.PickupsCanceledTotal.WithLabelValues(string(pickup.Type), "manual").Inc()
	log.InfoContext(ctx, "cancelled", slog.String("pickup_id", id.String()))
	return pickup, nil
}
