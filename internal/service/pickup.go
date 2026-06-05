package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
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
	hasPending, err := s.pickupRepo.HasPendingPaymentForHousehold(ctx, req.HouseholdID)
	if err != nil {
		return nil, err
	}
	if hasPending {
		return nil, fmt.Errorf("household has a pending payment: %w", domain.ErrConflict)
	}

	pickup := &domain.WastePickup{
		HouseholdID: req.HouseholdID,
		Type:        req.Type,
		Status:      domain.PickupStatusPending,
		SafetyCheck: req.SafetyCheck,
	}
	if err := s.pickupRepo.Create(ctx, pickup); err != nil {
		return nil, err
	}
	return pickup, nil
}

func (s *pickupService) List(ctx context.Context, filter domain.PickupFilter) ([]*domain.WastePickup, int, error) {
	return s.pickupRepo.List(ctx, filter)
}

// Schedule enforces BR-02 (must be pending) and BR-03 (electronic requires safety_check=true).
func (s *pickupService) Schedule(ctx context.Context, id uuid.UUID, req domain.SchedulePickupRequest) (*domain.WastePickup, error) {
	pickup, err := s.pickupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if pickup.Status != domain.PickupStatusPending {
		return nil, fmt.Errorf("pickup cannot be scheduled: current status is %s: %w", pickup.Status, domain.ErrConflict)
	}

	if pickup.Type == domain.WasteTypeElectronic && !pickup.SafetyCheck {
		return nil, fmt.Errorf("electronic pickup requires safety_check to be true before scheduling: %w", domain.ErrBusinessRule)
	}

	if err := s.pickupRepo.Schedule(ctx, id, req.PickupDate); err != nil {
		return nil, err
	}

	pickup.Status = domain.PickupStatusScheduled
	pickup.PickupDate = &req.PickupDate
	return pickup, nil
}

// Complete enforces BR-05: completing a pickup atomically creates its payment record.
func (s *pickupService) Complete(ctx context.Context, id uuid.UUID) (*domain.WastePickup, error) {
	pickup, err := s.pickupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if pickup.Status != domain.PickupStatusScheduled {
		return nil, fmt.Errorf("pickup status is %s, must be scheduled: %w", pickup.Status, domain.ErrConflict)
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
		return nil, fmt.Errorf("begin tx: %w", domain.ErrInternalFailure)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = s.pickupRepo.UpdateStatus(ctx, id, domain.PickupStatusCompleted, tx); err != nil {
		return nil, err
	}
	if err = s.paymentRepo.CreateWithTx(ctx, tx, payment); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", domain.ErrInternalFailure)
	}

	pickup.Status = domain.PickupStatusCompleted
	return pickup, nil
}

// Cancel transitions a pickup to canceled status if it is in a cancellable state.
func (s *pickupService) Cancel(ctx context.Context, id uuid.UUID) (*domain.WastePickup, error) {
	pickup, err := s.pickupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	switch pickup.Status {
	case domain.PickupStatusCompleted:
		return nil, fmt.Errorf("pickup status is completed, cannot cancel: %w", domain.ErrConflict)
	case domain.PickupStatusCanceled:
		return nil, fmt.Errorf("pickup is already canceled: %w", domain.ErrConflict)
	case domain.PickupStatusPending, domain.PickupStatusScheduled:
		// cancellable statuses — proceed
	}

	if err := s.pickupRepo.UpdateStatus(ctx, id, domain.PickupStatusCanceled); err != nil {
		return nil, err
	}

	pickup.Status = domain.PickupStatusCanceled
	return pickup, nil
}
