//go:build e2e

package e2e_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"time"
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

func (s *E2ESuite) TestWasteSummary_WithData() {
	// Create two organic and one plastic pickup so aggregation can be verified.
	var hResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Waste Summary Owner",
		"address":    "Jl. Waste Summary No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	for _, wt := range []string{"organic", "organic", "plastic"} {
		r := s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": wt})
		s.Require().Equal(http.StatusCreated, r.StatusCode)
		r.Body.Close()
	}

	resp = s.do(http.MethodGet, "/api/reports/waste-summary", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	var result map[string]any
	s.decode(resp, &result)
	s.True(result["success"].(bool))

	byType := make(map[string]any)
	for _, entry := range result["data"].([]any) {
		e := entry.(map[string]any)
		byType[e["type"].(string)] = e
	}

	organicEntry, ok := byType["organic"]
	s.Require().True(ok, "organic type must be present in waste summary")
	organicTotal := int(organicEntry.(map[string]any)["total"].(float64))
	s.GreaterOrEqual(organicTotal, 2, "expected at least 2 organic pickups in summary")

	plasticEntry, ok := byType["plastic"]
	s.Require().True(ok, "plastic type must be present in waste summary")
	plasticTotal := int(plasticEntry.(map[string]any)["total"].(float64))
	s.GreaterOrEqual(plasticTotal, 1, "expected at least 1 plastic pickup in summary")

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestPaymentSummary_WithData() {
	var hResp, pResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Payment Summary Owner",
		"address":    "Jl. Payment Summary No. 1",
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

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("proof", "receipt.jpg")
	s.Require().NoError(err)
	_, err = part.Write([]byte("fake-jpeg-data"))
	s.Require().NoError(err)
	mw.Close()
	req, err := http.NewRequest(http.MethodPut, s.baseURL+pathf("/api/payments/%s/confirm", paymentID), &buf)
	s.Require().NoError(err)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	confirmResp, err := s.client.Do(req)
	s.Require().NoError(err)
	confirmResp.Body.Close()
	s.Require().Equal(http.StatusOK, confirmResp.StatusCode)

	resp = s.do(http.MethodGet, "/api/reports/payment-summary", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	var summaryResult map[string]any
	s.decode(resp, &summaryResult)
	s.True(summaryResult["success"].(bool))

	summaryData := summaryResult["data"].(map[string]any)
	byStatusList := summaryData["by_status"].([]any)
	byStatus := make(map[string]any)
	for _, entry := range byStatusList {
		e := entry.(map[string]any)
		byStatus[e["status"].(string)] = e
	}
	paidEntry, ok := byStatus["paid"]
	s.Require().True(ok, "paid status must appear in payment summary")
	paidCount := int(paidEntry.(map[string]any)["count"].(float64))
	s.GreaterOrEqual(paidCount, 1, "expected at least 1 paid payment in summary")

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestHouseholdHistory_WithPickupsAndPayments() {
	var hResp, pResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "History Full Owner",
		"address":    "Jl. History Full No. 1",
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

	resp = s.do(http.MethodGet, pathf("/api/reports/households/%s/history", householdID), nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	var histResp map[string]any
	s.decode(resp, &histResp)
	s.True(histResp["success"].(bool))

	histData := histResp["data"].(map[string]any)
	household := histData["household"].(map[string]any)
	s.Equal(householdID, household["id"])

	pickups := histData["pickups"].([]any)
	s.Require().NotEmpty(pickups)
	pickupIDs := make(map[string]bool)
	for _, p := range pickups {
		pickupIDs[p.(map[string]any)["id"].(string)] = true
	}
	s.True(pickupIDs[pickupID], "pickup must appear in household history")

	histPayments := histData["payments"].([]any)
	s.Require().NotEmpty(histPayments)
	paymentIDs := make(map[string]bool)
	for _, p := range histPayments {
		paymentIDs[p.(map[string]any)["id"].(string)] = true
	}
	s.True(paymentIDs[paymentID], "payment must appear in household history")

	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}
