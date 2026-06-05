//go:build e2e

package e2e_test

import (
	"net/http"
	"time"
)

func (s *E2ESuite) TestOrganicWorker_BR04_AutoCancel() {
	if s.db == nil {
		s.T().Skip("E2E_DB_URL not set")
	}

	// Create household
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Worker BR04 Owner",
		"address":    "Jl. Worker No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	// Create organic pickup
	var pResp map[string]any
	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{
		"household_id": householdID,
		"type":         "organic",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	// Backdate created_at to 4 days ago (exceeds the 3-day cutoff)
	s.execDB(
		`UPDATE waste_pickups SET created_at = NOW() - INTERVAL '4 days' WHERE id = $1`,
		pickupID,
	)

	// Wait for the worker to tick (WORKER_CANCEL_INTERVAL=5s in CI; 7s gives a full margin)
	time.Sleep(7 * time.Second)

	// List pickups for this household and find ours
	resp = s.do(http.MethodGet, pathf("/api/pickups?household_id=%s", householdID), nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp map[string]any
	s.decode(resp, &listResp)
	items := listResp["data"].([]any)

	var status string
	for _, item := range items {
		p := item.(map[string]any)
		if p["id"].(string) == pickupID {
			status = p["status"].(string)
			break
		}
	}
	s.Equal("canceled", status, "expected organic pickup older than cutoff to be auto-canceled by worker")

	// Cleanup
	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}
