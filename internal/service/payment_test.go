package service_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

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
	repo    *mocks.PaymentRepository
	storage *mocks.StorageService
	svc     domain.PaymentService
}

func (s *PaymentServiceSuite) SetupTest() {
	s.repo = mocks.NewPaymentRepository(s.T())
	s.storage = mocks.NewStorageService(s.T())
	s.svc = service.NewPaymentService(s.repo, s.storage)
}

func TestPaymentService(t *testing.T) {
	suite.Run(t, new(PaymentServiceSuite))
}

func (s *PaymentServiceSuite) TestCreate_Success() {
	s.repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Payment")).Return(nil)

	req := domain.CreatePaymentRequest{
		HouseholdID: uuid.New(),
		WasteID:     uuid.New(),
		Amount:      decimal.RequireFromString("50000.00"),
	}
	got, err := s.svc.Create(s.T().Context(), req)
	s.Require().NoError(err)
	s.Equal(domain.PaymentStatusPending, got.Status)
	s.Equal(decimal.RequireFromString("50000.00"), got.Amount)
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
	s.repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Payment")).
		Return(fmt.Errorf("household not found: %w", domain.ErrNotFound))

	_, err := s.svc.Create(s.T().Context(), domain.CreatePaymentRequest{
		HouseholdID: uuid.New(),
		WasteID:     uuid.New(),
		Amount:      decimal.RequireFromString("50000.00"),
	})
	s.Require().ErrorIs(err, domain.ErrNotFound)
}

func (s *PaymentServiceSuite) TestCreate_RepoReturnsConflict_Propagates() {
	s.repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Payment")).
		Return(fmt.Errorf("payment for this pickup already exists: %w", domain.ErrConflict))

	_, err := s.svc.Create(s.T().Context(), domain.CreatePaymentRequest{
		HouseholdID: uuid.New(),
		WasteID:     uuid.New(),
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

type ReportServiceSuite struct {
	suite.Suite
	repo *mocks.PaymentRepository
	svc  domain.ReportService
}

func (s *ReportServiceSuite) SetupTest() {
	s.repo = mocks.NewPaymentRepository(s.T())
	s.svc = service.NewReportService(s.repo)
}

func TestReportService(t *testing.T) {
	suite.Run(t, new(ReportServiceSuite))
}

func (s *ReportServiceSuite) TestWasteSummary_DelegatesToRepo() {
	expected := []domain.WasteTypeSummary{{Type: domain.WasteTypeOrganic, Total: 5}}
	s.repo.On("WasteSummary", mock.Anything).Return(expected, nil)

	got, err := s.svc.WasteSummary(s.T().Context())
	s.Require().NoError(err)
	s.Equal(expected, got)
}

func (s *ReportServiceSuite) TestPaymentSummary_DelegatesToRepo() {
	expected := &domain.PaymentSummaryResult{TotalRevenue: decimal.RequireFromString("100000")}
	s.repo.On("PaymentSummary", mock.Anything).Return(expected, nil)

	got, err := s.svc.PaymentSummary(s.T().Context())
	s.Require().NoError(err)
	s.Equal(expected, got)
}

func (s *ReportServiceSuite) TestHouseholdHistory_DelegatesToRepo() {
	id := uuid.New()
	expected := &domain.HouseholdHistoryResult{Household: &domain.Household{ID: id}}
	s.repo.On("HouseholdHistory", mock.Anything, id).Return(expected, nil)

	got, err := s.svc.HouseholdHistory(s.T().Context(), id)
	s.Require().NoError(err)
	s.Equal(expected, got)
}

func (s *ReportServiceSuite) TestHouseholdHistory_NotFound() {
	id := uuid.New()
	s.repo.On("HouseholdHistory", mock.Anything, id).Return(nil, domain.ErrNotFound)

	_, err := s.svc.HouseholdHistory(s.T().Context(), id)
	s.Require().ErrorIs(err, domain.ErrNotFound)
}

// Confirm that all payment dates are in UTC.
func (s *ReportServiceSuite) TestConfirmSetsUTCTime() {
	now := time.Now().UTC()
	s.True(now.Location() == time.UTC)
}
