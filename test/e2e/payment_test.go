//go:build e2e

package e2e_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"time"
)

func (s *E2ESuite) TestPayment_ConfirmWithProof() {
	// Create household and complete a pickup to generate a pending payment
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Payment E2E Owner",
		"address":    "Jl. Payment No. 1",
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

	s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.do(http.MethodPut, pathf("/api/pickups/%s/complete", pickupID), nil)

	// List payments to find the pending one
	resp = s.do(http.MethodGet, "/api/payments", nil)
	s.Equal(http.StatusOK, resp.StatusCode)
	var payList map[string]any
	s.decode(resp, &payList)
	payments := payList["data"].([]any)
	s.Require().NotEmpty(payments, "expected at least one pending payment")

	var paymentID string
	for _, p := range payments {
		pm := p.(map[string]any)
		if pm["household_id"] == householdID && pm["status"] == "pending" {
			paymentID = pm["id"].(string)
			break
		}
	}
	s.Require().NotEmpty(paymentID, "no pending payment found for household")

	// Upload proof file
	body, contentType, err := jpegProofBody()
	s.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPut, s.baseURL+pathf("/api/payments/%s/confirm", paymentID), body)
	s.Require().NoError(err)
	req.Header.Set("Content-Type", contentType)
	confirmResp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer confirmResp.Body.Close()
	s.Equal(http.StatusOK, confirmResp.StatusCode)

	// Cleanup
	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestListPayments_200() {
	resp := s.do(http.MethodGet, "/api/payments", nil)
	s.Equal(http.StatusOK, resp.StatusCode)
}

func (s *E2ESuite) TestPayment_DirectCreate() {
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Direct Pay Owner",
		"address":    "Jl. Direct Pay No. 1",
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
	wasteID := pResp["data"].(map[string]any)["id"].(string)

	var payResp map[string]any
	resp = s.do(http.MethodPost, "/api/payments", map[string]any{
		"household_id": householdID,
		"waste_id":     wasteID,
		"amount":       "50000.00",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &payResp)
	data := payResp["data"].(map[string]any)
	s.NotEmpty(data["id"])
	s.Equal(householdID, data["household_id"])
	s.Equal(wasteID, data["waste_id"])
	s.Equal("50000.00", data["amount"])
	s.Equal("pending", data["status"])

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPayment_FilterByStatus() {
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Pay Filter Status Owner",
		"address":    "Jl. Pay Filter No. 1",
	})
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
	s.do(http.MethodPut, pathf("/api/pickups/%s/complete", pickupID), nil)

	// The completed pickup auto-creates a pending payment; filter by status=pending
	resp = s.do(http.MethodGet, fmt.Sprintf("/api/payments?status=pending&household_id=%s", householdID), nil)
	s.Equal(http.StatusOK, resp.StatusCode)
	var listResp map[string]any
	s.decode(resp, &listResp)
	data := listResp["data"].([]any)
	s.Require().NotEmpty(data)
	for _, item := range data {
		s.Equal("pending", item.(map[string]any)["status"])
	}

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPayment_FilterByHousehold() {
	var h1Resp, h2Resp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{"owner_name": "Pay HH1", "address": "Jl. Pay HH1"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &h1Resp)
	h1ID := h1Resp["data"].(map[string]any)["id"].(string)

	resp = s.do(http.MethodPost, "/api/households", map[string]any{"owner_name": "Pay HH2", "address": "Jl. Pay HH2"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &h2Resp)
	h2ID := h2Resp["data"].(map[string]any)["id"].(string)

	// Create and complete a pickup for each household to generate auto-payments
	for _, hID := range []string{h1ID, h2ID} {
		var pr map[string]any
		r := s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": hID, "type": "plastic"})
		s.Require().Equal(http.StatusCreated, r.StatusCode)
		s.decode(r, &pr)
		pid := pr["data"].(map[string]any)["id"].(string)
		s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pid), map[string]any{
			"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		})
		s.do(http.MethodPut, pathf("/api/pickups/%s/complete", pid), nil)
	}

	resp = s.do(http.MethodGet, fmt.Sprintf("/api/payments?household_id=%s", h1ID), nil)
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

func (s *E2ESuite) TestPayment_FilterByDateRange() {
	var hResp, pResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Date Range Owner",
		"address":    "Jl. Date Range No. 1",
	})
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

	// Find and confirm the pending payment
	var listResp map[string]any
	resp = s.do(http.MethodGet, fmt.Sprintf("/api/payments?household_id=%s", householdID), nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.decode(resp, &listResp)
	payments := listResp["data"].([]any)
	s.Require().NotEmpty(payments)
	paymentID := payments[0].(map[string]any)["id"].(string)

	body, contentType, err := jpegProofBody()
	s.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPut, s.baseURL+pathf("/api/payments/%s/confirm", paymentID), body)
	s.Require().NoError(err)
	req.Header.Set("Content-Type", contentType)
	confirmResp, err := s.client.Do(req)
	s.Require().NoError(err)
	confirmResp.Body.Close()
	s.Require().Equal(http.StatusOK, confirmResp.StatusCode)

	// Query with date range spanning now
	yesterday := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	tomorrow := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	resp = s.do(http.MethodGet, fmt.Sprintf("/api/payments?date_from=%s&date_to=%s&household_id=%s",
		yesterday, tomorrow, householdID), nil)
	s.Equal(http.StatusOK, resp.StatusCode)
	var rangeResp map[string]any
	s.decode(resp, &rangeResp)
	rangeData := rangeResp["data"].([]any)
	s.Require().NotEmpty(rangeData, "expected paid payment in date range")

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPayment_ConfirmNonExistent() {
	// Service calls FindByID first; non-existent ID returns ErrNotFound → 404.
	body, contentType, err := jpegProofBody()
	s.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPut,
		s.baseURL+"/api/payments/00000000-0000-0000-0000-000000000001/confirm", body)
	s.Require().NoError(err)
	req.Header.Set("Content-Type", contentType)
	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *E2ESuite) TestPayment_ProofFileURLPopulated() {
	var hResp, pResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Proof URL Owner",
		"address":    "Jl. Proof No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "organic"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.do(http.MethodPut, pathf("/api/pickups/%s/complete", pickupID), nil)

	var listResp map[string]any
	resp = s.do(http.MethodGet, fmt.Sprintf("/api/payments?household_id=%s", householdID), nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.decode(resp, &listResp)
	payments := listResp["data"].([]any)
	s.Require().NotEmpty(payments)
	paymentID := payments[0].(map[string]any)["id"].(string)

	body, contentType, err := jpegProofBody()
	s.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPut, s.baseURL+pathf("/api/payments/%s/confirm", paymentID), body)
	s.Require().NoError(err)
	req.Header.Set("Content-Type", contentType)
	confirmResp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer confirmResp.Body.Close()
	s.Require().Equal(http.StatusOK, confirmResp.StatusCode)

	var confirmBody map[string]any
	s.decode(confirmResp, &confirmBody)
	data := confirmBody["data"].(map[string]any)
	s.Equal("paid", data["status"])
	s.NotEmpty(data["proof_file_url"], "proof_file_url must be populated after confirm")

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPayment_AmountByType_Organic() {
	var hResp, pResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Organic Amount Owner",
		"address":    "Jl. Organic No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "organic"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.do(http.MethodPut, pathf("/api/pickups/%s/complete", pickupID), nil)

	var listResp map[string]any
	resp = s.do(http.MethodGet, fmt.Sprintf("/api/payments?household_id=%s", householdID), nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.decode(resp, &listResp)
	payments := listResp["data"].([]any)
	s.Require().NotEmpty(payments)
	s.Equal("50000.00", payments[0].(map[string]any)["amount"])

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPayment_AmountByType_Electronic() {
	var hResp, pResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Electronic Amount Owner",
		"address":    "Jl. Electronic No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{
		"household_id": householdID,
		"type":         "electronic",
		"safety_check": true,
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.do(http.MethodPut, pathf("/api/pickups/%s/complete", pickupID), nil)

	var listResp map[string]any
	resp = s.do(http.MethodGet, fmt.Sprintf("/api/payments?household_id=%s", householdID), nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.decode(resp, &listResp)
	payments := listResp["data"].([]any)
	s.Require().NotEmpty(payments)
	s.Equal("100000.00", payments[0].(map[string]any)["amount"])

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPayment_CreateNonExistentHousehold() {
	// db_exists_household validator catches this at the handler layer → 400
	resp := s.do(http.MethodPost, "/api/payments", map[string]any{
		"household_id": "00000000-0000-0000-0000-000000000001",
		"waste_id":     "00000000-0000-0000-0000-000000000002",
		"amount":       "50000.00",
	})
	s.Equal(http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func (s *E2ESuite) TestPayment_CreateNonExistentWasteID() {
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "No Waste Owner",
		"address":    "Jl. No Waste No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	// db_exists_pickup validator catches non-existent waste_id at handler layer → 400
	resp = s.do(http.MethodPost, "/api/payments", map[string]any{
		"household_id": householdID,
		"waste_id":     "00000000-0000-0000-0000-000000000002",
		"amount":       "50000.00",
	})
	s.Equal(http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

// TestPickup_CompletePaper_PaymentAmount50000 verifies that completing a paper
// pickup auto-creates a payment of 50 000 (same flat rate as organic/plastic).
func (s *E2ESuite) TestPickup_CompletePaper_PaymentAmount50000() {
	var hResp, pResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Paper Amount Owner",
		"address":    "Jl. Paper No. 1",
	})
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

	var listResp map[string]any
	resp = s.do(http.MethodGet, fmt.Sprintf("/api/payments?household_id=%s", householdID), nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.decode(resp, &listResp)
	payments := listResp["data"].([]any)
	s.Require().NotEmpty(payments)
	s.Equal("50000.00", payments[0].(map[string]any)["amount"])

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

// TestPickup_CompletePlastic_PaymentAmount50000 verifies that completing a
// plastic pickup auto-creates a payment of 50 000.
func (s *E2ESuite) TestPickup_CompletePlastic_PaymentAmount50000() {
	var hResp, pResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Plastic Amount Owner",
		"address":    "Jl. Plastic No. 1",
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

	var listResp map[string]any
	resp = s.do(http.MethodGet, fmt.Sprintf("/api/payments?household_id=%s", householdID), nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.decode(resp, &listResp)
	payments := listResp["data"].([]any)
	s.Require().NotEmpty(payments)
	s.Equal("50000.00", payments[0].(map[string]any)["amount"])

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

// TestPayment_FilterByAllThree_StatusHouseholdDateRange verifies that combining
// status + household_id + date_from/date_to filters returns only matching records.
func (s *E2ESuite) TestPayment_FilterByAllThree_StatusHouseholdDateRange() {
	var hResp, pResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Triple Filter Owner",
		"address":    "Jl. Triple Filter No. 1",
	})
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

	// Find the auto-created pending payment and confirm it.
	var listResp map[string]any
	resp = s.do(http.MethodGet, fmt.Sprintf("/api/payments?household_id=%s", householdID), nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.decode(resp, &listResp)
	payments := listResp["data"].([]any)
	s.Require().NotEmpty(payments)
	paymentID := payments[0].(map[string]any)["id"].(string)
	s.confirmPayment(paymentID)

	// Query with all three filters spanning now — should find the paid payment.
	yesterday := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	tomorrow := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	url := fmt.Sprintf("/api/payments?status=paid&household_id=%s&date_from=%s&date_to=%s",
		householdID, yesterday, tomorrow)
	resp = s.do(http.MethodGet, url, nil)
	s.Equal(http.StatusOK, resp.StatusCode)
	var filtResp map[string]any
	s.decode(resp, &filtResp)
	data := filtResp["data"].([]any)
	s.Require().NotEmpty(data, "expected at least one paid payment within date range")
	for _, item := range data {
		pm := item.(map[string]any)
		s.Equal(householdID, pm["household_id"])
		s.Equal("paid", pm["status"])
	}

	// Query with wrong status — should return empty.
	url2 := fmt.Sprintf("/api/payments?status=pending&household_id=%s&date_from=%s&date_to=%s",
		householdID, yesterday, tomorrow)
	resp = s.do(http.MethodGet, url2, nil)
	s.Equal(http.StatusOK, resp.StatusCode)
	var filtResp2 map[string]any
	s.decode(resp, &filtResp2)
	s.Empty(filtResp2["data"].([]any), "no pending payments should exist after confirmation")

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPayment_CreateDuplicateForSamePickup() {
	var hResp, pResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Dup Pay Owner",
		"address":    "Jl. Dup Pay No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "organic"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	wasteID := pResp["data"].(map[string]any)["id"].(string)

	resp = s.do(http.MethodPost, "/api/payments", map[string]any{
		"household_id": householdID, "waste_id": wasteID, "amount": "50000.00",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Second payment for the same pickup must conflict (waste_id UNIQUE)
	resp = s.do(http.MethodPost, "/api/payments", map[string]any{
		"household_id": householdID, "waste_id": wasteID, "amount": "50000.00",
	})
	s.Equal(http.StatusConflict, resp.StatusCode)
	resp.Body.Close()

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

// E1: PUT /api/payments/:id/confirm with a multipart body that omits the
// "proof" part must return 400 VALIDATION_ERROR (BR-06 E2E gap).
func (s *E2ESuite) TestConfirmPayment_MissingProofPart_400() {
	// Create household + pickup + complete to get a pending payment.
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "E1 Missing Proof Owner",
		"address":    "Jl. E1 No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)
	defer s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)

	var pResp map[string]any
	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{
		"household_id": householdID,
		"type":         "organic",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.do(http.MethodPut, pathf("/api/pickups/%s/complete", pickupID), nil)

	// Find the newly created pending payment.
	var payList map[string]any
	resp = s.do(http.MethodGet, "/api/payments", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.decode(resp, &payList)
	var paymentID string
	for _, p := range payList["data"].([]any) {
		pm := p.(map[string]any)
		if pm["household_id"] == householdID && pm["status"] == "pending" {
			paymentID = pm["id"].(string)
			break
		}
	}
	s.Require().NotEmpty(paymentID, "no pending payment found for household")

	// Submit a multipart body with no "proof" part — just an unrelated field.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("unrelated", "value")
	mw.Close()

	req, err := http.NewRequest(
		http.MethodPut,
		s.baseURL+pathf("/api/payments/%s/confirm", paymentID),
		&body,
	)
	s.Require().NoError(err)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	confirmResp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer confirmResp.Body.Close()
	s.Equal(http.StatusBadRequest, confirmResp.StatusCode)
}

// TestPayment_Confirm_ProofFileURLIsValidHTTP verifies that after a successful
// payment confirmation the returned proof_file_url is an absolute HTTP(S) URL
// pointing at the configured S3 endpoint (BR-06 storage proof).
func (s *E2ESuite) TestPayment_Confirm_ProofFileURLIsValidHTTP() {
	var hResp, pResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Proof URL Format Owner",
		"address":    "Jl. Proof URL No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)
	defer s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)

	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "organic"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.do(http.MethodPut, pathf("/api/pickups/%s/complete", pickupID), nil)

	var listResp map[string]any
	r := s.do(http.MethodGet, fmt.Sprintf("/api/payments?household_id=%s", householdID), nil)
	s.Require().Equal(http.StatusOK, r.StatusCode)
	s.decode(r, &listResp)
	payments := listResp["data"].([]any)
	s.Require().NotEmpty(payments)
	paymentID := payments[0].(map[string]any)["id"].(string)

	body, contentType, err := jpegProofBody()
	s.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPut, s.baseURL+pathf("/api/payments/%s/confirm", paymentID), body)
	s.Require().NoError(err)
	req.Header.Set("Content-Type", contentType)
	confirmResp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer confirmResp.Body.Close()
	s.Require().Equal(http.StatusOK, confirmResp.StatusCode)

	var confirmBody map[string]any
	s.decode(confirmResp, &confirmBody)
	data := confirmBody["data"].(map[string]any)
	s.Equal("paid", data["status"])

	proofURL, ok := data["proof_file_url"].(string)
	s.Require().True(ok, "proof_file_url must be a string")
	s.Require().NotEmpty(proofURL, "proof_file_url must be non-empty after confirm")
	isHTTP := len(proofURL) >= 7 && proofURL[:7] == "http://"
	isHTTPS := len(proofURL) >= 8 && proofURL[:8] == "https://"
	s.True(isHTTP || isHTTPS, "proof_file_url must be an absolute HTTP(S) URL, got: %s", proofURL)
}
