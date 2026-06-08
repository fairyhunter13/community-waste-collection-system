package service_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/mocks"
	"github.com/fairyhunter13/community-waste-collection-system/internal/service"
)

type PaymentServiceSuite struct {
	suite.Suite
	repo       *mocks.PaymentRepository
	pickupRepo *mocks.PickupRepository
	storage    *mocks.StorageService
	svc        domain.PaymentService
}

func (s *PaymentServiceSuite) SetupTest() {
	s.repo = mocks.NewPaymentRepository(s.T())
	s.pickupRepo = mocks.NewPickupRepository(s.T())
	s.storage = mocks.NewStorageService(s.T())
	s.svc = service.NewPaymentService(s.repo, s.pickupRepo, s.storage)
}

func TestPaymentService(t *testing.T) {
	suite.Run(t, new(PaymentServiceSuite))
}

func (s *PaymentServiceSuite) TestCreate_Success() {
	hid := uuid.New()
	wid := uuid.New()
	s.pickupRepo.On("FindByID", mock.Anything, wid).Return(&domain.WastePickup{
		ID:          wid,
		HouseholdID: hid,
		Type:        domain.WasteTypePlastic, // canonical amount: 50000.00
	}, nil)
	s.repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Payment")).Return(nil)

	req := domain.CreatePaymentRequest{
		HouseholdID: hid,
		WasteID:     wid,
		Amount:      decimal.RequireFromString("1.00"), // ignored; server derives canonical amount
	}
	got, err := s.svc.Create(s.T().Context(), req)
	s.Require().NoError(err)
	s.Equal(domain.PaymentStatusPending, got.Status)
	s.Equal(decimal.RequireFromString("50000.00"), got.Amount)
}

func (s *PaymentServiceSuite) TestCreate_CrossHousehold_Rejected() {
	wid := uuid.New()
	s.pickupRepo.On("FindByID", mock.Anything, wid).Return(&domain.WastePickup{
		ID:          wid,
		HouseholdID: uuid.New(), // different household
	}, nil)

	req := domain.CreatePaymentRequest{
		HouseholdID: uuid.New(), // foreign household
		WasteID:     wid,
		Amount:      decimal.RequireFromString("50000.00"),
	}
	_, err := s.svc.Create(s.T().Context(), req)
	s.Require().ErrorIs(err, domain.ErrBusinessRule)
}

func (s *PaymentServiceSuite) TestConfirm_BR06_NilFileReturnsValidationError() {
	_, err := s.svc.Confirm(s.T().Context(), uuid.New(), nil, 0, "image/jpeg")
	s.Require().ErrorIs(err, domain.ErrValidation)
}

func (s *PaymentServiceSuite) TestConfirm_AlreadyPaid_ReturnsConflict() {
	id := uuid.New()
	s.repo.On("FindByID", mock.Anything, id).Return(&domain.Payment{
		ID:     id,
		Status: domain.PaymentStatusPaid,
	}, nil)

	_, err := s.svc.Confirm(s.T().Context(), id, bytes.NewReader([]byte("data")), 4, "image/jpeg")
	s.Require().ErrorIs(err, domain.ErrConflict)
}

func (s *PaymentServiceSuite) TestConfirm_Success() {
	id := uuid.New()
	proofURL := "http://localhost:9000/waste-proofs/proof.jpg"

	s.repo.On("FindByID", mock.Anything, id).Return(&domain.Payment{
		ID:     id,
		Status: domain.PaymentStatusPending,
	}, nil)
	s.storage.On("Upload", mock.Anything, mock.AnythingOfType("string"),
		mock.Anything, int64(4), "image/jpeg").
		Return(proofURL, nil)
	s.repo.On("Confirm", mock.Anything, id, proofURL, mock.AnythingOfType("time.Time")).Return(nil)

	got, err := s.svc.Confirm(s.T().Context(), id, bytes.NewReader([]byte("data")), 4, "image/jpeg")
	s.Require().NoError(err)
	s.Equal(domain.PaymentStatusPaid, got.Status)
	s.Require().NotNil(got.ProofFileURL)
	s.Equal(proofURL, *got.ProofFileURL)
	s.Require().NotNil(got.PaymentDate)
}

func (s *PaymentServiceSuite) TestConfirm_StorageError_Propagates() {
	id := uuid.New()

	s.repo.On("FindByID", mock.Anything, id).Return(&domain.Payment{
		ID:     id,
		Status: domain.PaymentStatusPending,
	}, nil)
	s.storage.On("Upload", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything).Return("", domain.ErrInternalFailure)

	_, err := s.svc.Confirm(s.T().Context(), id, bytes.NewReader([]byte("data")), 4, "image/jpeg")
	s.Require().ErrorIs(err, domain.ErrInternalFailure)
}

func (s *PaymentServiceSuite) TestCreate_RepoReturnsNotFound_Propagates() {
	hid := uuid.New()
	wid := uuid.New()
	s.pickupRepo.On("FindByID", mock.Anything, wid).Return(&domain.WastePickup{
		ID: wid, HouseholdID: hid,
	}, nil)
	s.repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Payment")).
		Return(fmt.Errorf("household not found: %w", domain.ErrNotFound))

	_, err := s.svc.Create(s.T().Context(), domain.CreatePaymentRequest{
		HouseholdID: hid,
		WasteID:     wid,
		Amount:      decimal.RequireFromString("50000.00"),
	})
	s.Require().ErrorIs(err, domain.ErrNotFound)
}

func (s *PaymentServiceSuite) TestCreate_RepoReturnsConflict_Propagates() {
	hid := uuid.New()
	wid := uuid.New()
	s.pickupRepo.On("FindByID", mock.Anything, wid).Return(&domain.WastePickup{
		ID: wid, HouseholdID: hid,
	}, nil)
	s.repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Payment")).
		Return(fmt.Errorf("payment for this pickup already exists: %w", domain.ErrConflict))

	_, err := s.svc.Create(s.T().Context(), domain.CreatePaymentRequest{
		HouseholdID: hid,
		WasteID:     wid,
		Amount:      decimal.RequireFromString("50000.00"),
	})
	s.Require().ErrorIs(err, domain.ErrConflict)
}

func (s *PaymentServiceSuite) TestList_DelegatesToRepo() {
	payments := []*domain.Payment{{ID: uuid.New()}}
	filter := domain.PaymentFilter{Page: 1, PerPage: 20}
	s.repo.On("List", mock.Anything, filter).Return(payments, 1, nil)

	got, total, err := s.svc.List(s.T().Context(), filter)
	s.Require().NoError(err)
	s.Equal(1, total)
	s.Len(got, 1)
}

func (s *PaymentServiceSuite) TestList_RepoError_Propagates() {
	filter := domain.PaymentFilter{Page: 1, PerPage: 20}
	s.repo.On("List", mock.Anything, filter).
		Return(([]*domain.Payment)(nil), 0, domain.ErrInternalFailure)

	_, _, err := s.svc.List(s.T().Context(), filter)
	s.Require().ErrorIs(err, domain.ErrInternalFailure)
}
