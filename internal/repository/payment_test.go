//go:build integration

package repository_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/suite"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/repository"
)

type PaymentRepoSuite struct {
	baseRepoSuite
	repo          domain.PaymentRepository
	householdRepo domain.HouseholdRepository
	pickupRepo    domain.PickupRepository
}

func (s *PaymentRepoSuite) SetupSuite() {
	s.baseRepoSuite.SetupSuite()
	s.repo = repository.NewPaymentRepository(s.db)
	s.householdRepo = repository.NewHouseholdRepository(s.db)
	s.pickupRepo = repository.NewPickupRepository(s.db)
}

func TestPaymentRepository(t *testing.T) {
	suite.Run(t, new(PaymentRepoSuite))
}

func (s *PaymentRepoSuite) newHousehold() *domain.Household {
	h := &domain.Household{OwnerName: "Test Owner", Address: "Test Address"}
	s.Require().NoError(s.householdRepo.Create(s.T().Context(), h))
	return h
}

func (s *PaymentRepoSuite) newPickup(householdID uuid.UUID) *domain.WastePickup {
	p := &domain.WastePickup{
		HouseholdID: householdID,
		Type:        domain.WasteTypeOrganic,
		Status:      domain.PickupStatusPending,
	}
	s.Require().NoError(s.pickupRepo.Create(s.T().Context(), p))
	return p
}

func (s *PaymentRepoSuite) newPayment(householdID, wasteID uuid.UUID) *domain.Payment {
	p := &domain.Payment{
		HouseholdID: householdID,
		WasteID:     wasteID,
		Amount:      decimal.RequireFromString("50000.00"),
		Status:      domain.PaymentStatusPending,
	}
	s.Require().NoError(s.repo.Create(s.T().Context(), p))
	return p
}

func (s *PaymentRepoSuite) TestCreate_SetsIDAndTimestamps() {
	h := s.newHousehold()
	p := s.newPickup(h.ID)
	payment := s.newPayment(h.ID, p.ID)

	s.Require().NotEqual(uuid.Nil, payment.ID)
	s.Equal(domain.PaymentStatusPending, payment.Status)
	s.Nil(payment.PaymentDate)
	s.Nil(payment.ProofFileURL)
}

func (s *PaymentRepoSuite) TestFindByID_Found() {
	h := s.newHousehold()
	p := s.newPickup(h.ID)
	payment := s.newPayment(h.ID, p.ID)

	got, err := s.repo.FindByID(s.T().Context(), payment.ID)
	s.Require().NoError(err)
	s.Equal(payment.ID, got.ID)
	s.Equal(decimal.RequireFromString("50000.00"), got.Amount)
}

func (s *PaymentRepoSuite) TestFindByID_NotFound() {
	_, err := s.repo.FindByID(s.T().Context(), uuid.New())
	s.Require().ErrorIs(err, domain.ErrNotFound)
}

func (s *PaymentRepoSuite) TestCreateWithTx_Atomic() {
	h := s.newHousehold()
	pickup := s.newPickup(h.ID)

	tx, err := s.db.Beginx()
	s.Require().NoError(err)

	payment := &domain.Payment{
		HouseholdID: h.ID,
		WasteID:     pickup.ID,
		Amount:      decimal.RequireFromString("50000.00"),
		Status:      domain.PaymentStatusPending,
	}
	s.Require().NoError(s.repo.CreateWithTx(s.T().Context(), tx, payment))
	s.Require().NoError(tx.Commit())

	got, err := s.repo.FindByID(s.T().Context(), payment.ID)
	s.Require().NoError(err)
	s.Equal(payment.ID, got.ID)
}

func (s *PaymentRepoSuite) TestConfirm_UpdatesStatusAndProof() {
	h := s.newHousehold()
	p := s.newPickup(h.ID)
	payment := s.newPayment(h.ID, p.ID)

	paidAt := time.Now().UTC().Truncate(time.Second)
	proofURL := "http://localhost:9000/waste-proofs/proof.jpg"
	s.Require().NoError(s.repo.Confirm(s.T().Context(), payment.ID, proofURL, paidAt))

	got, err := s.repo.FindByID(s.T().Context(), payment.ID)
	s.Require().NoError(err)
	s.Equal(domain.PaymentStatusPaid, got.Status)
	s.Require().NotNil(got.ProofFileURL)
	s.Equal(proofURL, *got.ProofFileURL)
	s.Require().NotNil(got.PaymentDate)
}

func (s *PaymentRepoSuite) TestList_FilterByStatus() {
	h := s.newHousehold()
	p1 := s.newPickup(h.ID)
	p2 := s.newPickup(h.ID)
	// The partial UNIQUE index permits at most one pending payment per
	// household, so confirm pay1 (→ paid) before inserting pay2.
	pay1 := s.newPayment(h.ID, p1.ID)
	s.Require().NoError(s.repo.Confirm(s.T().Context(), pay1.ID, "http://example.com/proof.jpg", time.Now()))
	_ = s.newPayment(h.ID, p2.ID)

	status := domain.PaymentStatusPending
	payments, total, err := s.repo.List(s.T().Context(), domain.PaymentFilter{
		Status:  &status,
		Page:    1,
		PerPage: 20,
	})
	s.Require().NoError(err)
	s.Equal(1, total)
	s.Equal(domain.PaymentStatusPending, payments[0].Status)
}

func (s *PaymentRepoSuite) TestList_FilterByDateRange() {
	h := s.newHousehold()
	p1 := s.newPickup(h.ID)
	p2 := s.newPickup(h.ID)

	// Per the partial UNIQUE index, only one pending payment may exist per
	// household. Confirm each before inserting the next.
	pay1 := s.newPayment(h.ID, p1.ID)
	s.Require().NoError(s.repo.Confirm(s.T().Context(), pay1.ID, "http://example.com/1.jpg", time.Now()))
	pay2 := s.newPayment(h.ID, p2.ID)
	s.Require().NoError(s.repo.Confirm(s.T().Context(), pay2.ID, "http://example.com/2.jpg", time.Now()))

	yesterday := time.Now().Add(-24 * time.Hour)
	tomorrow := time.Now().Add(24 * time.Hour)

	payments, total, err := s.repo.List(s.T().Context(), domain.PaymentFilter{
		DateFrom: &yesterday,
		DateTo:   &tomorrow,
		Page:     1,
		PerPage:  20,
	})
	s.Require().NoError(err)
	s.Equal(2, total)
	s.Len(payments, 2)
}

func (s *PaymentRepoSuite) TestWasteSummary_AggregatesCorrectly() {
	h := s.newHousehold()

	// Create 2 organic pending pickups.
	for range 2 {
		_ = s.newPickup(h.ID)
	}

	// Create 1 plastic pickup.
	plastic := &domain.WastePickup{HouseholdID: h.ID, Type: domain.WasteTypePlastic, Status: domain.PickupStatusPending}
	s.Require().NoError(s.pickupRepo.Create(s.T().Context(), plastic))

	summaries, err := s.repo.WasteSummary(s.T().Context())
	s.Require().NoError(err)

	byType := make(map[domain.WasteType]domain.WasteTypeSummary)
	for _, summary := range summaries {
		byType[summary.Type] = summary
	}

	s.Equal(2, byType[domain.WasteTypeOrganic].Total)
	s.Equal(1, byType[domain.WasteTypePlastic].Total)
	s.Equal(2, byType[domain.WasteTypeOrganic].ByStatus[string(domain.PickupStatusPending)])
}

func (s *PaymentRepoSuite) TestPaymentSummary_CountsAndRevenue() {
	h := s.newHousehold()
	p1 := s.newPickup(h.ID)
	p2 := s.newPickup(h.ID)
	// Confirm pay1 (→ paid) before inserting pay2 so the household has at
	// most one pending payment at a time, per the partial UNIQUE index.
	pay1 := s.newPayment(h.ID, p1.ID)
	s.Require().NoError(s.repo.Confirm(s.T().Context(), pay1.ID, "http://example.com/proof.jpg", time.Now()))
	_ = s.newPayment(h.ID, p2.ID)

	result, err := s.repo.PaymentSummary(s.T().Context())
	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.NotEmpty(result.ByStatus)

	byStatus := make(map[domain.PaymentStatus]domain.PaymentStatusSummary)
	for _, r := range result.ByStatus {
		byStatus[r.Status] = r
	}
	s.Equal(1, byStatus[domain.PaymentStatusPaid].Count)
	s.Equal(1, byStatus[domain.PaymentStatusPending].Count)
}

func (s *PaymentRepoSuite) TestHouseholdHistory_ReturnsCompleteHistory() {
	h := s.newHousehold()
	pickup := s.newPickup(h.ID)
	payment := s.newPayment(h.ID, pickup.ID)

	history, err := s.repo.HouseholdHistory(s.T().Context(), h.ID)
	s.Require().NoError(err)
	s.Require().NotNil(history)
	s.Equal(h.ID, history.Household.ID)
	s.Require().Len(history.Pickups, 1)
	s.Equal(pickup.ID, history.Pickups[0].ID)
	s.Require().Len(history.Payments, 1)
	s.Equal(payment.ID, history.Payments[0].ID)
}

func (s *PaymentRepoSuite) TestHouseholdHistory_NotFound() {
	_, err := s.repo.HouseholdHistory(s.T().Context(), uuid.New())
	s.Require().ErrorIs(err, domain.ErrNotFound)
}

func (s *PaymentRepoSuite) TestConfirm_AlreadyPaid_ReturnsConflict() {
	h := s.newHousehold()
	p := s.newPickup(h.ID)
	payment := s.newPayment(h.ID, p.ID)

	proofURL := "http://localhost:9000/waste-proofs/proof1.jpg"
	s.Require().NoError(s.repo.Confirm(s.T().Context(), payment.ID, proofURL, time.Now()))

	// Second confirm on the same payment should return ErrConflict.
	err := s.repo.Confirm(s.T().Context(), payment.ID, "http://localhost:9000/waste-proofs/proof2.jpg", time.Now())
	s.Require().ErrorIs(err, domain.ErrConflict)
}

func (s *PaymentRepoSuite) TestList_FilterByHousehold() {
	h1 := s.newHousehold()
	h2 := s.newHousehold()
	p1 := s.newPickup(h1.ID)
	p2 := s.newPickup(h2.ID)
	pay1 := s.newPayment(h1.ID, p1.ID)
	_ = s.newPayment(h2.ID, p2.ID)

	payments, total, err := s.repo.List(s.T().Context(), domain.PaymentFilter{
		HouseholdID: &h1.ID,
		Page:        1,
		PerPage:     20,
	})
	s.Require().NoError(err)
	s.Equal(1, total)
	s.Equal(pay1.ID, payments[0].ID)
}

func (s *PaymentRepoSuite) TestCreate_NonExistentHousehold_ReturnsNotFound() {
	p := &domain.Payment{
		HouseholdID: uuid.New(), // does not exist
		WasteID:     uuid.New(),
		Amount:      decimal.RequireFromString("50000.00"),
		Status:      domain.PaymentStatusPending,
	}
	err := s.repo.Create(s.T().Context(), p)
	s.Require().ErrorIs(err, domain.ErrNotFound)
}

func (s *PaymentRepoSuite) TestCreate_NonExistentWasteID_ReturnsNotFound() {
	h := s.newHousehold()
	p := &domain.Payment{
		HouseholdID: h.ID,
		WasteID:     uuid.New(), // does not exist
		Amount:      decimal.RequireFromString("50000.00"),
		Status:      domain.PaymentStatusPending,
	}
	err := s.repo.Create(s.T().Context(), p)
	s.Require().ErrorIs(err, domain.ErrNotFound)
}

// TestList_AllThreeFilters_Combined verifies that status + household_id + date_range
// filters are applied together, returning only records matching all three criteria.
func (s *PaymentRepoSuite) TestList_AllThreeFilters_Combined() {
	h1 := s.newHousehold()
	h2 := s.newHousehold()
	p1 := s.newPickup(h1.ID)
	p2 := s.newPickup(h1.ID)
	p3 := s.newPickup(h2.ID)

	// Confirm pay1 (→ paid) before inserting pay2 so the h1 household has at
	// most one pending payment outstanding at any time, per the partial
	// UNIQUE index.
	now := time.Now().UTC()
	pay1 := s.newPayment(h1.ID, p1.ID) // h1, pending
	s.Require().NoError(s.repo.Confirm(s.T().Context(), pay1.ID, "http://example.com/1.jpg", now))
	_ = s.newPayment(h1.ID, p2.ID) // h1, pending  (will stay pending)
	_ = s.newPayment(h2.ID, p3.ID) // h2, pending

	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)
	status := domain.PaymentStatusPaid

	payments, total, err := s.repo.List(s.T().Context(), domain.PaymentFilter{
		HouseholdID: &h1.ID,
		Status:      &status,
		DateFrom:    &yesterday,
		DateTo:      &tomorrow,
		Page:        1,
		PerPage:     20,
	})
	s.Require().NoError(err)
	s.Equal(1, total)
	s.Require().Len(payments, 1)
	s.Equal(pay1.ID, payments[0].ID)
	s.Equal(domain.PaymentStatusPaid, payments[0].Status)
	s.Equal(h1.ID, payments[0].HouseholdID)
}

// TestPaymentSummary_AllStatuses verifies that the payment summary includes
// pending, paid, and failed status entries, and that total_revenue contains
// only the sum of paid payment amounts.
func (s *PaymentRepoSuite) TestPaymentSummary_AllStatuses() {
	h := s.newHousehold()
	p1 := s.newPickup(h.ID)
	p2 := s.newPickup(h.ID)
	p3 := &domain.WastePickup{HouseholdID: h.ID, Type: domain.WasteTypeOrganic, Status: domain.PickupStatusPending}
	s.Require().NoError(s.pickupRepo.Create(s.T().Context(), p3))

	// paid payment — confirm first so the household has no pending payment
	// while we go on to create the pending and failed rows. The partial
	// UNIQUE index permits exactly one pending payment per household.
	pay2 := s.newPayment(h.ID, p2.ID)
	s.Require().NoError(s.repo.Confirm(s.T().Context(), pay2.ID, "http://example.com/2.jpg", time.Now()))

	// failed payment — direct insert with status=failed (not constrained
	// by the partial pending-only index).
	failedPay := &domain.Payment{
		HouseholdID: h.ID,
		WasteID:     p3.ID,
		Amount:      decimal.RequireFromString("50000.00"),
		Status:      domain.PaymentStatusFailed,
	}
	s.Require().NoError(s.repo.Create(s.T().Context(), failedPay))

	// pending payment — only insert after the paid row is in place.
	_ = s.newPayment(h.ID, p1.ID)

	result, err := s.repo.PaymentSummary(s.T().Context())
	s.Require().NoError(err)
	s.Require().NotNil(result)

	byStatus := make(map[domain.PaymentStatus]domain.PaymentStatusSummary)
	for _, r := range result.ByStatus {
		byStatus[r.Status] = r
	}

	s.GreaterOrEqual(byStatus[domain.PaymentStatusPending].Count, 1)
	s.GreaterOrEqual(byStatus[domain.PaymentStatusPaid].Count, 1)
	s.GreaterOrEqual(byStatus[domain.PaymentStatusFailed].Count, 1)

	// total_revenue must only sum paid amounts — at least 50 000.
	s.True(result.TotalRevenue.GreaterThanOrEqual(decimal.RequireFromString("50000.00")),
		"total_revenue must be at least 50000 (from the one paid payment)")
}

// TestList_DateRange_NoMatches_ReturnsEmpty verifies that a date range with no
// confirmed payments returns an empty slice without error.
func (s *PaymentRepoSuite) TestList_DateRange_NoMatches_ReturnsEmpty() {
	h := s.newHousehold()
	p := s.newPickup(h.ID)
	_ = s.newPayment(h.ID, p.ID) // pending, no payment_date

	// Date range well in the future — no confirmed payments exist in that window.
	future1 := time.Now().Add(365 * 24 * time.Hour)
	future2 := time.Now().Add(366 * 24 * time.Hour)

	payments, total, err := s.repo.List(s.T().Context(), domain.PaymentFilter{
		DateFrom: &future1,
		DateTo:   &future2,
		Page:     1,
		PerPage:  20,
	})
	s.Require().NoError(err)
	s.Equal(0, total)
	s.Empty(payments)
}

func (s *PaymentRepoSuite) TestCreate_DuplicateWasteID_ReturnsConflict() {
	h := s.newHousehold()
	pickup := s.newPickup(h.ID)
	_ = s.newPayment(h.ID, pickup.ID)

	// Second payment for the same pickup should fail (waste_id is UNIQUE).
	p2 := &domain.Payment{
		HouseholdID: h.ID,
		WasteID:     pickup.ID,
		Amount:      decimal.RequireFromString("50000.00"),
		Status:      domain.PaymentStatusPending,
	}
	err := s.repo.Create(s.T().Context(), p2)
	s.Require().ErrorIs(err, domain.ErrConflict)
}
