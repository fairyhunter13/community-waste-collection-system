//go:build integration

package repository_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/suite"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/repository"
)

type PickupRepoSuite struct {
	baseRepoSuite
	repo          domain.PickupRepository
	householdRepo domain.HouseholdRepository
}

func (s *PickupRepoSuite) SetupSuite() {
	s.baseRepoSuite.SetupSuite()
	s.repo = repository.NewPickupRepository(s.db)
	s.householdRepo = repository.NewHouseholdRepository(s.db)
}

func TestPickupRepository(t *testing.T) {
	suite.Run(t, new(PickupRepoSuite))
}

func (s *PickupRepoSuite) newHousehold() *domain.Household {
	h := &domain.Household{OwnerName: "Test Owner", Address: "Test Address"}
	s.Require().NoError(s.householdRepo.Create(s.T().Context(), h))
	return h
}

func (s *PickupRepoSuite) newPickup(householdID uuid.UUID, wasteType domain.WasteType) *domain.WastePickup {
	p := &domain.WastePickup{
		HouseholdID: householdID,
		Type:        wasteType,
		Status:      domain.PickupStatusPending,
	}
	s.Require().NoError(s.repo.Create(s.T().Context(), p))
	return p
}

// insertAgedPickup creates a pickup and back-dates its created_at for worker tests.
func (s *PickupRepoSuite) insertAgedPickup(householdID uuid.UUID, daysOld int) *domain.WastePickup {
	p := s.newPickup(householdID, domain.WasteTypeOrganic)
	_, err := s.db.ExecContext(s.T().Context(),
		`UPDATE waste_pickups SET created_at = NOW() - $1::interval WHERE id = $2`,
		fmt.Sprintf("%d days", daysOld),
		p.ID,
	)
	s.Require().NoError(err)
	return p
}

func (s *PickupRepoSuite) TestCreate_DefaultsPendingStatus() {
	h := s.newHousehold()
	p := s.newPickup(h.ID, domain.WasteTypeOrganic)

	s.Require().NotEqual(uuid.Nil, p.ID)
	s.Equal(domain.PickupStatusPending, p.Status)
	s.False(p.SafetyCheck)
	s.Nil(p.PickupDate)
}

func (s *PickupRepoSuite) TestFindByID_Found() {
	h := s.newHousehold()
	p := s.newPickup(h.ID, domain.WasteTypePlastic)

	got, err := s.repo.FindByID(s.T().Context(), p.ID)
	s.Require().NoError(err)
	s.Equal(p.ID, got.ID)
	s.Equal(domain.WasteTypePlastic, got.Type)
}

func (s *PickupRepoSuite) TestFindByID_NotFound() {
	_, err := s.repo.FindByID(s.T().Context(), uuid.New())
	s.Require().ErrorIs(err, domain.ErrNotFound)
}

func (s *PickupRepoSuite) TestList_FilterByStatus() {
	h := s.newHousehold()
	p1 := s.newPickup(h.ID, domain.WasteTypeOrganic)
	p2 := s.newPickup(h.ID, domain.WasteTypePaper)

	status := domain.PickupStatusPending
	s.Require().NoError(s.repo.UpdateStatus(s.T().Context(), p2.ID, domain.PickupStatusCanceled))

	filter := domain.PickupFilter{Status: &status, Page: 1, PerPage: 20}
	pickups, total, err := s.repo.List(s.T().Context(), filter)
	s.Require().NoError(err)
	s.Equal(1, total)
	s.Equal(p1.ID, pickups[0].ID)
}

func (s *PickupRepoSuite) TestList_FilterByHousehold() {
	h1 := s.newHousehold()
	h2 := s.newHousehold()
	s.newPickup(h1.ID, domain.WasteTypeOrganic)
	s.newPickup(h2.ID, domain.WasteTypePlastic)

	filter := domain.PickupFilter{HouseholdID: &h1.ID, Page: 1, PerPage: 20}
	pickups, total, err := s.repo.List(s.T().Context(), filter)
	s.Require().NoError(err)
	s.Equal(1, total)
	s.Equal(h1.ID, pickups[0].HouseholdID)
}

func (s *PickupRepoSuite) TestSchedule_SetsDateAndStatus() {
	h := s.newHousehold()
	p := s.newPickup(h.ID, domain.WasteTypeOrganic)

	pickupDate := time.Now().Add(48 * time.Hour)
	s.Require().NoError(s.repo.Schedule(s.T().Context(), p.ID, pickupDate))

	got, err := s.repo.FindByID(s.T().Context(), p.ID)
	s.Require().NoError(err)
	s.Equal(domain.PickupStatusScheduled, got.Status)
	s.Require().NotNil(got.PickupDate)
}

func (s *PickupRepoSuite) TestUpdateStatus_ChangesStatus() {
	h := s.newHousehold()
	p := s.newPickup(h.ID, domain.WasteTypeElectronic)

	s.Require().NoError(s.repo.UpdateStatus(s.T().Context(), p.ID, domain.PickupStatusCanceled))

	got, err := s.repo.FindByID(s.T().Context(), p.ID)
	s.Require().NoError(err)
	s.Equal(domain.PickupStatusCanceled, got.Status)
}

func (s *PickupRepoSuite) TestHasPendingPaymentForHousehold_True() {
	h := s.newHousehold()
	p := s.newPickup(h.ID, domain.WasteTypeOrganic)

	// Insert a pending payment for this household.
	paymentRepo := repository.NewPaymentRepository(s.db)
	payment := &domain.Payment{
		HouseholdID: h.ID,
		WasteID:     p.ID,
		Amount:      decimal.RequireFromString("50000.00"),
		Status:      domain.PaymentStatusPending,
	}
	s.Require().NoError(paymentRepo.Create(s.T().Context(), payment))

	hasPending, err := s.repo.HasPendingPaymentForHousehold(s.T().Context(), h.ID)
	s.Require().NoError(err)
	s.True(hasPending)
}

func (s *PickupRepoSuite) TestHasPendingPaymentForHousehold_False() {
	h := s.newHousehold()
	hasPending, err := s.repo.HasPendingPaymentForHousehold(s.T().Context(), h.ID)
	s.Require().NoError(err)
	s.False(hasPending)
}

func (s *PickupRepoSuite) TestFindExpiredOrganic_ReturnsOnlyExpired() {
	h := s.newHousehold()
	expired := s.insertAgedPickup(h.ID, 5)         // 5 days old
	_ = s.newPickup(h.ID, domain.WasteTypeOrganic) // fresh
	_ = s.newPickup(h.ID, domain.WasteTypePlastic) // different type

	cutoff := time.Now().Add(-3 * 24 * time.Hour) // 3 days ago
	results, err := s.repo.FindExpiredOrganic(s.T().Context(), cutoff)
	s.Require().NoError(err)
	s.Require().Len(results, 1)
	s.Equal(expired.ID, results[0].ID)
}

func (s *PickupRepoSuite) TestBulkCancel_CancelsAll() {
	h := s.newHousehold()
	p1 := s.newPickup(h.ID, domain.WasteTypeOrganic)
	p2 := s.newPickup(h.ID, domain.WasteTypeOrganic)

	s.Require().NoError(s.repo.BulkCancel(s.T().Context(), []uuid.UUID{p1.ID, p2.ID}))

	for _, id := range []uuid.UUID{p1.ID, p2.ID} {
		got, err := s.repo.FindByID(s.T().Context(), id)
		s.Require().NoError(err)
		s.Equal(domain.PickupStatusCanceled, got.Status)
	}
}

func (s *PickupRepoSuite) TestBulkCancel_EmptySlice_NoError() {
	s.Require().NoError(s.repo.BulkCancel(s.T().Context(), nil))
}

func (s *PickupRepoSuite) TestList_FilterByStatusAndHousehold() {
	h1 := s.newHousehold()
	h2 := s.newHousehold()
	p1 := s.newPickup(h1.ID, domain.WasteTypeOrganic) // pending, h1
	_ = s.newPickup(h1.ID, domain.WasteTypePlastic)   // pending, h1 — cancel it
	_ = s.newPickup(h2.ID, domain.WasteTypeOrganic)   // pending, h2

	s.Require().NoError(s.repo.UpdateStatus(s.T().Context(), p1.ID, domain.PickupStatusScheduled))

	status := domain.PickupStatusScheduled
	filter := domain.PickupFilter{HouseholdID: &h1.ID, Status: &status, Page: 1, PerPage: 20}
	pickups, total, err := s.repo.List(s.T().Context(), filter)
	s.Require().NoError(err)
	s.Equal(1, total)
	s.Equal(p1.ID, pickups[0].ID)
}

func (s *PickupRepoSuite) TestCancelIfCancellable_PendingPickup() {
	h := s.newHousehold()
	p := s.newPickup(h.ID, domain.WasteTypeOrganic)

	ok, err := s.repo.CancelIfCancellable(s.T().Context(), p.ID)
	s.Require().NoError(err)
	s.True(ok)

	got, err := s.repo.FindByID(s.T().Context(), p.ID)
	s.Require().NoError(err)
	s.Equal(domain.PickupStatusCanceled, got.Status)
}

func (s *PickupRepoSuite) TestCancelIfCancellable_CompletedPickup() {
	h := s.newHousehold()
	p := s.newPickup(h.ID, domain.WasteTypeOrganic)
	s.Require().NoError(s.repo.UpdateStatus(s.T().Context(), p.ID, domain.PickupStatusCompleted))

	ok, err := s.repo.CancelIfCancellable(s.T().Context(), p.ID)
	s.Require().NoError(err)
	s.False(ok)

	got, err := s.repo.FindByID(s.T().Context(), p.ID)
	s.Require().NoError(err)
	s.Equal(domain.PickupStatusCompleted, got.Status)
}

func (s *PickupRepoSuite) TestCancelIfCancellable_NonExistentID() {
	ok, err := s.repo.CancelIfCancellable(s.T().Context(), uuid.New())
	s.Require().NoError(err)
	s.False(ok)
}

// TestFindExpiredOrganic_OnlyExpiredPendingOrganic verifies that FindExpiredOrganic
// excludes scheduled organic pickups and aged non-organic pickups — only expired
// pending organic pickups are returned.
func (s *PickupRepoSuite) TestFindExpiredOrganic_OnlyExpiredPendingOrganic() {
	h := s.newHousehold()

	// Pending organic, aged past cutoff — must be returned.
	expiredOrganic := s.insertAgedPickup(h.ID, 5)

	// Scheduled organic, aged past cutoff — must NOT be returned (not pending).
	scheduledOrganic := s.newPickup(h.ID, domain.WasteTypeOrganic)
	s.Require().NoError(s.repo.Schedule(s.T().Context(), scheduledOrganic.ID, time.Now().Add(24*time.Hour)))
	_, err := s.db.ExecContext(s.T().Context(),
		`UPDATE waste_pickups SET created_at = NOW() - '5 days'::interval WHERE id = $1`,
		scheduledOrganic.ID,
	)
	s.Require().NoError(err)

	// Pending plastic, aged past cutoff — must NOT be returned (wrong type).
	plasticPickup := &domain.WastePickup{
		HouseholdID: h.ID,
		Type:        domain.WasteTypePlastic,
		Status:      domain.PickupStatusPending,
	}
	s.Require().NoError(s.repo.Create(s.T().Context(), plasticPickup))
	_, err = s.db.ExecContext(s.T().Context(),
		`UPDATE waste_pickups SET created_at = NOW() - '5 days'::interval WHERE id = $1`,
		plasticPickup.ID,
	)
	s.Require().NoError(err)

	cutoff := time.Now().Add(-3 * 24 * time.Hour)
	results, err := s.repo.FindExpiredOrganic(s.T().Context(), cutoff)
	s.Require().NoError(err)

	ids := make(map[uuid.UUID]bool)
	for _, r := range results {
		ids[r.ID] = true
	}
	s.True(ids[expiredOrganic.ID], "pending expired organic must be returned")
	s.False(ids[scheduledOrganic.ID], "scheduled organic must not be returned")
	s.False(ids[plasticPickup.ID], "plastic pickup must not be returned")
}

func (s *PickupRepoSuite) TestCreate_NonExistentHousehold_ReturnsNotFound() {
	p := &domain.WastePickup{
		HouseholdID: uuid.New(), // does not exist
		Type:        domain.WasteTypeOrganic,
		Status:      domain.PickupStatusPending,
	}
	err := s.repo.Create(s.T().Context(), p)
	s.Require().ErrorIs(err, domain.ErrNotFound)
}
