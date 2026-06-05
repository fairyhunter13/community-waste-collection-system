//go:build e2e

package e2e_test

import (
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

func (s *E2ESuite) TestPickup_RateLimit() {
	// Burst 11 rapid pickup-create requests to trigger rate limiting (burst=10)
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "RateLimit Owner",
		"address":    "Jl. Rate No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	got429 := false
	for i := 0; i < 15; i++ {
		r := s.do(http.MethodPost, "/api/pickups", map[string]any{
			"household_id": householdID,
			"type":         "organic",
		})
		if r.StatusCode == http.StatusTooManyRequests {
			got429 = true
		}
		r.Body.Close()
	}
	s.True(got429, "expected at least one 429 after bursting the rate limiter")

	// Cleanup
	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}
