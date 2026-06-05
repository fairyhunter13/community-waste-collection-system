//go:build e2e

package e2e_test

import (
	"bytes"
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
	defer confirmResp.Body.Close()
	s.Equal(http.StatusOK, confirmResp.StatusCode)

	// Cleanup
	s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
}

func (s *E2ESuite) TestListPayments_200() {
	resp := s.do(http.MethodGet, "/api/payments", nil)
	s.Equal(http.StatusOK, resp.StatusCode)
}
