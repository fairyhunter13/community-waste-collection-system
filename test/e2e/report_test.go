//go:build e2e

package e2e_test

import (
	"net/http"
)

func (s *E2ESuite) TestWasteSummary_200() {
	resp := s.do(http.MethodGet, "/api/reports/waste-summary", nil)
	s.Equal(http.StatusOK, resp.StatusCode)
	var result map[string]any
	s.decode(resp, &result)
	s.True(result["success"].(bool))
	s.NotNil(result["data"])
}

func (s *E2ESuite) TestPaymentSummary_200() {
	resp := s.do(http.MethodGet, "/api/reports/payment-summary", nil)
	s.Equal(http.StatusOK, resp.StatusCode)
	var result map[string]any
	s.decode(resp, &result)
	s.True(result["success"].(bool))
}

func (s *E2ESuite) TestHouseholdHistory_404_UnknownID() {
	resp := s.do(http.MethodGet, "/api/reports/households/00000000-0000-0000-0000-000000000000/history", nil)
	s.Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *E2ESuite) TestHouseholdHistory_200() {
	// Create household
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "History Owner",
		"address":    "Jl. History No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	resp = s.do(http.MethodGet, pathf("/api/reports/households/%s/history", householdID), nil)
	s.Equal(http.StatusOK, resp.StatusCode)

	// Cleanup
	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}
