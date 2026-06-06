package service_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/mocks"
	"github.com/fairyhunter13/community-waste-collection-system/internal/service"
)

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

// ── WasteSummary ──────────────────────────────────────────────────────────────

func (s *ReportServiceSuite) TestWasteSummary_DelegatesToRepo() {
	expected := []domain.WasteTypeSummary{
		{Type: domain.WasteTypeOrganic, Total: 5},
		{Type: domain.WasteTypePlastic, Total: 3},
	}
	s.repo.On("WasteSummary", mock.Anything).Return(expected, nil)

	got, err := s.svc.WasteSummary(s.T().Context())
	s.Require().NoError(err)
	s.Equal(expected, got)
}

func (s *ReportServiceSuite) TestWasteSummary_RepoError_Propagates() {
	s.repo.On("WasteSummary", mock.Anything).
		Return(([]domain.WasteTypeSummary)(nil), domain.ErrInternalFailure)

	_, err := s.svc.WasteSummary(s.T().Context())
	s.Require().ErrorIs(err, domain.ErrInternalFailure)
}

func (s *ReportServiceSuite) TestWasteSummary_EmptyResult_NotNil() {
	s.repo.On("WasteSummary", mock.Anything).
		Return([]domain.WasteTypeSummary{}, nil)

	got, err := s.svc.WasteSummary(s.T().Context())
	s.Require().NoError(err)
	s.NotNil(got)
	s.Empty(got)
}

// ── PaymentSummary ────────────────────────────────────────────────────────────

func (s *ReportServiceSuite) TestPaymentSummary_DelegatesToRepo() {
	expected := &domain.PaymentSummaryResult{
		TotalRevenue: decimal.RequireFromString("250000.00"),
		ByStatus: []domain.PaymentStatusSummary{
			{Status: domain.PaymentStatusPaid, Count: 5, Revenue: decimal.RequireFromString("250000.00")},
			{Status: domain.PaymentStatusPending, Count: 2, Revenue: decimal.Zero},
		},
	}
	s.repo.On("PaymentSummary", mock.Anything).Return(expected, nil)

	got, err := s.svc.PaymentSummary(s.T().Context())
	s.Require().NoError(err)
	s.Equal(expected, got)
}

func (s *ReportServiceSuite) TestPaymentSummary_RepoError_Propagates() {
	s.repo.On("PaymentSummary", mock.Anything).
		Return((*domain.PaymentSummaryResult)(nil), domain.ErrInternalFailure)

	_, err := s.svc.PaymentSummary(s.T().Context())
	s.Require().ErrorIs(err, domain.ErrInternalFailure)
}

// ── HouseholdHistory ──────────────────────────────────────────────────────────

func (s *ReportServiceSuite) TestHouseholdHistory_DelegatesToRepo() {
	id := uuid.New()
	expected := &domain.HouseholdHistoryResult{
		Household: &domain.Household{ID: id, OwnerName: "Owner"},
		Pickups:   []*domain.WastePickup{{ID: uuid.New()}},
		Payments:  []*domain.Payment{{ID: uuid.New()}},
	}
	s.repo.On("HouseholdHistory", mock.Anything, id).Return(expected, nil)

	got, err := s.svc.HouseholdHistory(s.T().Context(), id)
	s.Require().NoError(err)
	s.Equal(expected, got)
	s.Equal(id, got.Household.ID)
	s.Len(got.Pickups, 1)
	s.Len(got.Payments, 1)
}

func (s *ReportServiceSuite) TestHouseholdHistory_NotFound_Propagates() {
	id := uuid.New()
	s.repo.On("HouseholdHistory", mock.Anything, id).
		Return((*domain.HouseholdHistoryResult)(nil), domain.ErrNotFound)

	_, err := s.svc.HouseholdHistory(s.T().Context(), id)
	s.Require().ErrorIs(err, domain.ErrNotFound)
}

func (s *ReportServiceSuite) TestHouseholdHistory_InternalFailure_Propagates() {
	id := uuid.New()
	s.repo.On("HouseholdHistory", mock.Anything, id).
		Return((*domain.HouseholdHistoryResult)(nil), domain.ErrInternalFailure)

	_, err := s.svc.HouseholdHistory(s.T().Context(), id)
	s.Require().ErrorIs(err, domain.ErrInternalFailure)
}
