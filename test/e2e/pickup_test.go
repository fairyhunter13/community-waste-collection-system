//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (s *E2ESuite) TestPickup_FullLifecycle() {
	// Create a household first
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Pickup Test Owner",
		"address":    "Jl. Pickup No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	// Create pickup
	var pResp map[string]any
	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{
		"household_id": householdID,
		"type":         "plastic",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)
	s.Equal("pending", pResp["data"].(map[string]any)["status"])

	// List pickups
	resp = s.do(http.MethodGet, "/api/pickups", nil)
	s.Equal(http.StatusOK, resp.StatusCode)

	// Schedule
	var schedResp map[string]any
	resp = s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.decode(resp, &schedResp)
	s.Equal("scheduled", schedResp["data"].(map[string]any)["status"])

	// Complete
	var compResp map[string]any
	resp = s.do(http.MethodPut, pathf("/api/pickups/%s/complete", pickupID), nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.decode(resp, &compResp)
	s.Equal("completed", compResp["data"].(map[string]any)["status"])

	// Cleanup household
	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPickup_BR01_BlockedByPendingPayment() {
	// Create household
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "BR01 Owner",
		"address":    "Jl. BR01 No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	// Create and complete first pickup to generate a pending payment
	var p1Resp map[string]any
	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{
		"household_id": householdID,
		"type":         "paper",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &p1Resp)
	p1ID := p1Resp["data"].(map[string]any)["id"].(string)

	// Schedule and complete to trigger payment creation
	s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", p1ID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.do(http.MethodPut, pathf("/api/pickups/%s/complete", p1ID), nil)

	// Second pickup should be blocked (BR-01)
	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{
		"household_id": householdID,
		"type":         "organic",
	})
	s.Equal(http.StatusConflict, resp.StatusCode)

	// Cleanup
	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPickup_BR03_ElectronicRequiresSafetyCheck() {
	// Create household
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "BR03 Owner",
		"address":    "Jl. BR03 No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	// Create electronic pickup without safety check
	var pResp map[string]any
	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{
		"household_id": householdID,
		"type":         "electronic",
		"safety_check": false,
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	// Schedule should fail with 422
	resp = s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.Equal(http.StatusUnprocessableEntity, resp.StatusCode)

	// Cleanup
	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPickup_Cancel() {
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Cancel Owner",
		"address":    "Jl. Cancel No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	var pResp map[string]any
	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{
		"household_id": householdID,
		"type":         "organic",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	resp = s.do(http.MethodPut, pathf("/api/pickups/%s/cancel", pickupID), nil)
	s.Equal(http.StatusOK, resp.StatusCode)

	// Cleanup
	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPickup_FilterByStatus() {
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Filter Status Owner",
		"address":    "Jl. Filter No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	// Create two pickups; schedule one.
	var p1Resp, p2Resp map[string]any
	r1 := s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "organic"})
	s.Require().Equal(http.StatusCreated, r1.StatusCode)
	s.decode(r1, &p1Resp)
	p1ID := p1Resp["data"].(map[string]any)["id"].(string)

	r2 := s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "plastic"})
	s.Require().Equal(http.StatusCreated, r2.StatusCode)
	s.decode(r2, &p2Resp)

	s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", p1ID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})

	resp = s.do(http.MethodGet, "/api/pickups?status=scheduled&household_id="+householdID, nil)
	s.Equal(http.StatusOK, resp.StatusCode)
	var listResp map[string]any
	s.decode(resp, &listResp)
	data := listResp["data"].([]any)
	s.Require().Len(data, 1)
	s.Equal("scheduled", data[0].(map[string]any)["status"])

	_ = p2Resp
	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPickup_FilterByHouseholdID() {
	var h1Resp, h2Resp map[string]any
	r1 := s.do(http.MethodPost, "/api/households", map[string]any{"owner_name": "Filter HH1", "address": "Jl. HH1"})
	s.Require().Equal(http.StatusCreated, r1.StatusCode)
	s.decode(r1, &h1Resp)
	h1ID := h1Resp["data"].(map[string]any)["id"].(string)

	r2 := s.do(http.MethodPost, "/api/households", map[string]any{"owner_name": "Filter HH2", "address": "Jl. HH2"})
	s.Require().Equal(http.StatusCreated, r2.StatusCode)
	s.decode(r2, &h2Resp)
	h2ID := h2Resp["data"].(map[string]any)["id"].(string)

	s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": h1ID, "type": "organic"})
	s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": h2ID, "type": "plastic"})

	resp := s.do(http.MethodGet, "/api/pickups?household_id="+h1ID, nil)
	s.Equal(http.StatusOK, resp.StatusCode)
	var listResp map[string]any
	s.decode(resp, &listResp)
	data := listResp["data"].([]any)
	s.Require().NotEmpty(data)
	for _, item := range data {
		s.Equal(h1ID, item.(map[string]any)["household_id"])
	}

	s.do(http.MethodDelete, pathf("/api/households/%s", h1ID), nil)
	s.do(http.MethodDelete, pathf("/api/households/%s", h2ID), nil)
}

func (s *E2ESuite) TestPickup_BR02_ScheduleAlreadyScheduled() {
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{"owner_name": "BR02 Owner", "address": "Jl. BR02"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	var pResp map[string]any
	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "organic"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})

	// Second schedule attempt on a scheduled pickup should fail (BR-02: must be pending).
	resp = s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.Equal(http.StatusConflict, resp.StatusCode)

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPickup_BR03_ElectronicWithSafetyCheck_CanSchedule() {
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{"owner_name": "BR03 Happy Owner", "address": "Jl. BR03 Happy"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	var pResp map[string]any
	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{
		"household_id": householdID,
		"type":         "electronic",
		"safety_check": true,
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	resp = s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.Equal(http.StatusOK, resp.StatusCode)

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPickup_CompleteAlreadyCompleted() {
	var hResp, pResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{"owner_name": "Double Complete", "address": "Jl. DC"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "paper"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.do(http.MethodPut, pathf("/api/pickups/%s/complete", pickupID), nil)

	resp = s.do(http.MethodPut, pathf("/api/pickups/%s/complete", pickupID), nil)
	s.Equal(http.StatusConflict, resp.StatusCode)

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPickup_CancelCompleted() {
	var hResp, pResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{"owner_name": "Cancel Completed", "address": "Jl. CC"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "plastic"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.do(http.MethodPut, pathf("/api/pickups/%s/complete", pickupID), nil)

	resp = s.do(http.MethodPut, pathf("/api/pickups/%s/cancel", pickupID), nil)
	s.Equal(http.StatusConflict, resp.StatusCode)

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPickup_CreateNonExistentHousehold() {
	// db_exists_household validator catches this at the handler layer → 400
	resp := s.do(http.MethodPost, "/api/pickups", map[string]any{
		"household_id": "00000000-0000-0000-0000-000000000001",
		"type":         "organic",
	})
	s.Equal(http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// TestPickup_BR01_MixedPayments_OnlyPendingBlocks confirms that BR-01 blocks
// new pickups while a pending payment exists and unblocks once it is confirmed,
// even after an earlier paid payment exists for the same household. The
// partial-unique index uq_payments_one_pending_per_household (migration 000004)
// caps concurrent pending payments at one, so this test sequences them:
// pay1 (pending → paid) then pay2 (pending → paid).
func (s *E2ESuite) TestPickup_BR01_MixedPayments_OnlyPendingBlocks() {
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "BR01 Mixed Owner",
		"address":    "Jl. BR01 Mixed No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	// First pickup + pending payment.
	var p1Resp, pay1Resp map[string]any
	r1 := s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "plastic"})
	s.Require().Equal(http.StatusCreated, r1.StatusCode)
	s.decode(r1, &p1Resp)
	p1ID := p1Resp["data"].(map[string]any)["id"].(string)

	r3 := s.do(http.MethodPost, "/api/payments", map[string]any{
		"household_id": householdID, "waste_id": p1ID, "amount": "50000.00",
	})
	s.Require().Equal(http.StatusCreated, r3.StatusCode)
	s.decode(r3, &pay1Resp)
	pay1ID := pay1Resp["data"].(map[string]any)["id"].(string)

	// BR-01 blocks new pickups while pay1 is pending.
	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "organic"})
	s.Equal(http.StatusConflict, resp.StatusCode)
	resp.Body.Close()

	// Confirm pay1 → paid. Pending count = 0 ⇒ second pickup allowed.
	s.confirmPayment(pay1ID)

	// Second pickup + pending payment (paid payment from pay1 must NOT block).
	var p2Resp, pay2Resp map[string]any
	r2 := s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "paper"})
	s.Require().Equal(http.StatusCreated, r2.StatusCode)
	s.decode(r2, &p2Resp)
	p2ID := p2Resp["data"].(map[string]any)["id"].(string)

	r4 := s.do(http.MethodPost, "/api/payments", map[string]any{
		"household_id": householdID, "waste_id": p2ID, "amount": "50000.00",
	})
	s.Require().Equal(http.StatusCreated, r4.StatusCode)
	s.decode(r4, &pay2Resp)
	pay2ID := pay2Resp["data"].(map[string]any)["id"].(string)

	// BR-01 again blocks while pay2 is pending — even though pay1 is paid.
	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "organic"})
	s.Equal(http.StatusConflict, resp.StatusCode)
	resp.Body.Close()

	// Confirm pay2 → paid. Pending count = 0 ⇒ third pickup must succeed.
	s.confirmPayment(pay2ID)
	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "organic"})
	s.Equal(http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

// TestPickup_BR02_ScheduleCompleted_Fails checks that scheduling an already
// completed pickup returns 409 (BR-02 requires pending status).
func (s *E2ESuite) TestPickup_BR02_ScheduleCompleted_Fails() {
	var hResp, pResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "BR02 Completed Owner",
		"address":    "Jl. BR02 Completed No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "plastic"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.do(http.MethodPut, pathf("/api/pickups/%s/complete", pickupID), nil)

	// Attempt to reschedule a completed pickup — must fail.
	resp = s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.Equal(http.StatusConflict, resp.StatusCode)
	resp.Body.Close()

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

// TestPickup_BR02_ScheduleCanceled_Fails checks that scheduling a canceled
// pickup returns 409 (BR-02 requires pending status).
func (s *E2ESuite) TestPickup_BR02_ScheduleCanceled_Fails() {
	var hResp, pResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "BR02 Canceled Owner",
		"address":    "Jl. BR02 Canceled No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "organic"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	s.do(http.MethodPut, pathf("/api/pickups/%s/cancel", pickupID), nil)

	// Attempt to schedule a canceled pickup — must fail.
	resp = s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.Equal(http.StatusConflict, resp.StatusCode)
	resp.Body.Close()

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPickup_ScheduleNonExistent_404() {
	resp := s.do(http.MethodPut, "/api/pickups/00000000-0000-0000-0000-000000000001/schedule", map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.Equal(http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func (s *E2ESuite) TestPickup_CompleteNonExistent_404() {
	resp := s.do(http.MethodPut, "/api/pickups/00000000-0000-0000-0000-000000000002/complete", nil)
	s.Equal(http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func (s *E2ESuite) TestPickup_CancelNonExistent_404() {
	resp := s.do(http.MethodPut, "/api/pickups/00000000-0000-0000-0000-000000000003/cancel", nil)
	s.Equal(http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

// TestPickup_ListEmpty_Returns200EmptyData verifies that listing pickups for a
// household with no pickups returns 200 with an empty data array, not an error.
func (s *E2ESuite) TestPickup_ListEmpty_Returns200EmptyData() {
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Empty Pickup Owner",
		"address":    "Jl. Empty No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	resp = s.do(http.MethodGet, fmt.Sprintf("/api/pickups?household_id=%s", householdID), nil)
	s.Equal(http.StatusOK, resp.StatusCode)
	var listResp map[string]any
	s.decode(resp, &listResp)
	data := listResp["data"].([]any)
	s.Empty(data, "expected empty data array for household with no pickups")

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPickup_RateLimit() {
	// Uses a dedicated X-Real-IP to isolate this test's bucket from other tests.
	// The rate limiter is per-IP, so exhausting the bucket for "192.0.2.1" does
	// not affect subsequent tests that run from 127.0.0.1.
	const testIP = "192.0.2.1"

	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "RateLimit Owner",
		"address":    "Jl. Rate No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	got429 := false
	body, _ := json.Marshal(map[string]any{"household_id": householdID, "type": "organic"})
	for i := 0; i < 60; i++ {
		req, err := http.NewRequest(http.MethodPost, s.baseURL+"/api/pickups", bytes.NewReader(body))
		s.Require().NoError(err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Real-IP", testIP)
		r, err := s.client.Do(req)
		s.Require().NoError(err)
		if r.StatusCode == http.StatusTooManyRequests {
			got429 = true
		}
		r.Body.Close()
	}
	s.True(got429, "expected at least one 429 after bursting the rate limiter")

	// Cleanup
	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}
