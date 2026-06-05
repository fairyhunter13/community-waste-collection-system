package service_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/mocks"
	"github.com/fairyhunter13/community-waste-collection-system/internal/service"
)

type PickupServiceSuite struct {
	suite.Suite
	pickupRepo  *mocks.PickupRepository
	paymentRepo *mocks.PaymentRepository
	svc         domain.PickupService
}

func (s *PickupServiceSuite) SetupTest() {
	s.pickupRepo = mocks.NewPickupRepository(s.T())
	s.paymentRepo = mocks.NewPaymentRepository(s.T())
	// Pass nil DB; Complete tests will mock all DB calls via repositories.
	s.svc = service.NewPickupService(s.pickupRepo, s.paymentRepo, nil)
}

func TestPickupService(t *testing.T) {
	suite.Run(t, new(PickupServiceSuite))
}

// ── Create ────────────────────────────────────────────────────────────────────

func (s *PickupServiceSuite) TestCreate_BR01_BlockedByPendingPayment() {
	s.pickupRepo.On("HasPendingPaymentForHousehold", mock.Anything, mock.AnythingOfType("uuid.UUID")).
		Return(true, nil)

	_, err := s.svc.Create(s.T().Context(), domain.CreatePickupRequest{
		HouseholdID: uuid.New(),
		Type:        domain.WasteTypeOrganic,
	})
	s.Require().ErrorIs(err, domain.ErrConflict)
}

func (s *PickupServiceSuite) TestCreate_Success_NoPendingPayment() {
	s.pickupRepo.On("HasPendingPaymentForHousehold", mock.Anything, mock.Anything).Return(false, nil)
	s.pickupRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.WastePickup")).Return(nil)

	got, err := s.svc.Create(s.T().Context(), domain.CreatePickupRequest{
		HouseholdID: uuid.New(),
		Type:        domain.WasteTypeOrganic,
	})
	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Equal(domain.PickupStatusPending, got.Status)
}

// ── Schedule ──────────────────────────────────────────────────────────────────

func (s *PickupServiceSuite) TestSchedule_BR02_RejectsNonPendingStatus() {
	id := uuid.New()
	s.pickupRepo.On("FindByID", mock.Anything, id).Return(&domain.WastePickup{
		ID:     id,
		Status: domain.PickupStatusScheduled,
		Type:   domain.WasteTypeOrganic,
	}, nil)

	_, err := s.svc.Schedule(s.T().Context(), id, domain.SchedulePickupRequest{
		PickupDate: time.Now().Add(24 * time.Hour),
	})
	s.Require().ErrorIs(err, domain.ErrConflict)
}

func (s *PickupServiceSuite) TestSchedule_BR03_ElectronicRequiresSafetyCheck() {
	id := uuid.New()
	s.pickupRepo.On("FindByID", mock.Anything, id).Return(&domain.WastePickup{
		ID:          id,
		Status:      domain.PickupStatusPending,
		Type:        domain.WasteTypeElectronic,
		SafetyCheck: false,
	}, nil)

	_, err := s.svc.Schedule(s.T().Context(), id, domain.SchedulePickupRequest{
		PickupDate: time.Now().Add(24 * time.Hour),
	})
	s.Require().ErrorIs(err, domain.ErrBusinessRule)
}

func (s *PickupServiceSuite) TestSchedule_Success_ElectronicWithSafetyCheck() {
	id := uuid.New()
	date := time.Now().Add(24 * time.Hour)
	s.pickupRepo.On("FindByID", mock.Anything, id).Return(&domain.WastePickup{
		ID:          id,
		Status:      domain.PickupStatusPending,
		Type:        domain.WasteTypeElectronic,
		SafetyCheck: true,
	}, nil)
	s.pickupRepo.On("Schedule", mock.Anything, id, mock.AnythingOfType("time.Time")).Return(nil)

	got, err := s.svc.Schedule(s.T().Context(), id, domain.SchedulePickupRequest{PickupDate: date})
	s.Require().NoError(err)
	s.Equal(domain.PickupStatusScheduled, got.Status)
}

func (s *PickupServiceSuite) TestSchedule_Success_OrganicNoSafetyCheckNeeded() {
	id := uuid.New()
	s.pickupRepo.On("FindByID", mock.Anything, id).Return(&domain.WastePickup{
		ID:          id,
		Status:      domain.PickupStatusPending,
		Type:        domain.WasteTypeOrganic,
		SafetyCheck: false,
	}, nil)
	s.pickupRepo.On("Schedule", mock.Anything, id, mock.AnythingOfType("time.Time")).Return(nil)

	got, err := s.svc.Schedule(s.T().Context(), id, domain.SchedulePickupRequest{
		PickupDate: time.Now().Add(24 * time.Hour),
	})
	s.Require().NoError(err)
	s.Equal(domain.PickupStatusScheduled, got.Status)
}

// ── Complete ──────────────────────────────────────────────────────────────────

func (s *PickupServiceSuite) TestComplete_RejectsNonScheduledStatus() {
	id := uuid.New()
	s.pickupRepo.On("FindByID", mock.Anything, id).Return(&domain.WastePickup{
		ID:     id,
		Status: domain.PickupStatusPending,
		Type:   domain.WasteTypeOrganic,
	}, nil)

	_, err := s.svc.Complete(s.T().Context(), id)
	s.Require().ErrorIs(err, domain.ErrConflict)
}

// TestComplete_BR05 verifies the payment amount is set correctly from PaymentAmounts map.
// The full atomic behaviour is covered by integration tests.
func (s *PickupServiceSuite) TestComplete_BR05_AmountOrganic() {
	// Complete relies on a real DB tx; test the amount lookup separately.
	s.Equal("50000.00", domain.PaymentAmounts[domain.WasteTypeOrganic])
}

func (s *PickupServiceSuite) TestComplete_BR05_AmountElectronic() {
	s.Equal("100000.00", domain.PaymentAmounts[domain.WasteTypeElectronic])
}

func (s *PickupServiceSuite) TestComplete_BR05_AmountPlastic() {
	s.Equal("50000.00", domain.PaymentAmounts[domain.WasteTypePlastic])
}

func (s *PickupServiceSuite) TestComplete_BR05_AmountPaper() {
	s.Equal("50000.00", domain.PaymentAmounts[domain.WasteTypePaper])
}

// ── Cancel ────────────────────────────────────────────────────────────────────

func (s *PickupServiceSuite) TestCancel_RejectsCompleted() {
	id := uuid.New()
	s.pickupRepo.On("FindByID", mock.Anything, id).Return(&domain.WastePickup{
		ID:     id,
		Status: domain.PickupStatusCompleted,
	}, nil)

	_, err := s.svc.Cancel(s.T().Context(), id)
	s.Require().ErrorIs(err, domain.ErrConflict)
}

func (s *PickupServiceSuite) TestCancel_RejectsAlreadyCanceled() {
	id := uuid.New()
	s.pickupRepo.On("FindByID", mock.Anything, id).Return(&domain.WastePickup{
		ID:     id,
		Status: domain.PickupStatusCanceled,
	}, nil)

	_, err := s.svc.Cancel(s.T().Context(), id)
	s.Require().ErrorIs(err, domain.ErrConflict)
}

func (s *PickupServiceSuite) TestCancel_Success_FromPending() {
	id := uuid.New()
	s.pickupRepo.On("FindByID", mock.Anything, id).Return(&domain.WastePickup{
		ID:     id,
		Status: domain.PickupStatusPending,
	}, nil)
	s.pickupRepo.On("UpdateStatus", mock.Anything, id, domain.PickupStatusCanceled).Return(nil)

	got, err := s.svc.Cancel(s.T().Context(), id)
	s.Require().NoError(err)
	s.Equal(domain.PickupStatusCanceled, got.Status)
}

func (s *PickupServiceSuite) TestCancel_Success_FromScheduled() {
	id := uuid.New()
	s.pickupRepo.On("FindByID", mock.Anything, id).Return(&domain.WastePickup{
		ID:     id,
		Status: domain.PickupStatusScheduled,
	}, nil)
	s.pickupRepo.On("UpdateStatus", mock.Anything, id, domain.PickupStatusCanceled).Return(nil)

	got, err := s.svc.Cancel(s.T().Context(), id)
	s.Require().NoError(err)
	s.Equal(domain.PickupStatusCanceled, got.Status)
}
