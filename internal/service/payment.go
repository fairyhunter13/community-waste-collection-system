package service

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
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
	payment := &domain.Payment{
		HouseholdID: req.HouseholdID,
		WasteID:     req.WasteID,
		Amount:      req.Amount,
		Status:      domain.PaymentStatusPending,
	}
	if err := s.repo.Create(ctx, payment); err != nil {
		return nil, err
	}
	return payment, nil
}

func (s *paymentService) List(ctx context.Context, filter domain.PaymentFilter) ([]*domain.Payment, int, error) {
	return s.repo.List(ctx, filter)
}

// Confirm enforces BR-06: a proof file is required to confirm a payment.
func (s *paymentService) Confirm(ctx context.Context, id uuid.UUID, file io.Reader, size int64, contentType string) (*domain.Payment, error) {
	if file == nil {
		return nil, fmt.Errorf("proof file is required: %w", domain.ErrValidation)
	}

	payment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if payment.Status != domain.PaymentStatusPending {
		return nil, fmt.Errorf("payment status is %s, must be pending: %w", payment.Status, domain.ErrConflict)
	}

	key := fmt.Sprintf("%s/proof_%s", id, uuid.New().String())
	proofURL, err := s.storage.Upload(ctx, key, file, size, contentType)
	if err != nil {
		return nil, err
	}

	paidAt := time.Now().UTC()
	if err := s.repo.Confirm(ctx, id, proofURL, paidAt); err != nil {
		return nil, err
	}

	payment.Status = domain.PaymentStatusPaid
	payment.ProofFileURL = &proofURL
	payment.PaymentDate = &paidAt
	return payment, nil
}
