//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"sync"
	"time"
)

// TestPayment_CrossHouseholdRejected proves that PaymentService.Create rejects a
// payment where waste_id belongs to a different household than household_id,
// returning 422 BUSINESS_RULE_VIOLATION with no payment row written.
// Covers Phase-10 fix F4.
func (s *E2ESuite) TestPayment_CrossHouseholdRejected() {
	var h1Resp, h2Resp map[string]any
	r := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Cross HH Owner A", "address": "Jl. Cross A No.1",
	})
	s.Require().Equal(http.StatusCreated, r.StatusCode)
	s.decode(r, &h1Resp)
	h1ID := h1Resp["data"].(map[string]any)["id"].(string)
	defer s.do(http.MethodDelete, pathf("/api/households/%s", h1ID), nil)

	r = s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Cross HH Owner B", "address": "Jl. Cross B No.1",
	})
	s.Require().Equal(http.StatusCreated, r.StatusCode)
	s.decode(r, &h2Resp)
	h2ID := h2Resp["data"].(map[string]any)["id"].(string)
	defer s.do(http.MethodDelete, pathf("/api/households/%s", h2ID), nil)

	// Create a pickup for household A.
	var pkResp map[string]any
	r = s.do(http.MethodPost, "/api/pickups", map[string]any{
		"household_id": h1ID, "type": "plastic",
	})
	s.Require().Equal(http.StatusCreated, r.StatusCode)
	s.decode(r, &pkResp)
	pickupID := pkResp["data"].(map[string]any)["id"].(string)

	// Attempt to bill household B for household A's pickup — must be rejected.
	r = s.do(http.MethodPost, "/api/payments", map[string]any{
		"household_id": h2ID,
		"waste_id":     pickupID,
		"amount":       "50000.00",
	})
	defer r.Body.Close()
	s.Require().Equal(http.StatusUnprocessableEntity, r.StatusCode)

	var body map[string]any
	s.Require().NoError(json.NewDecoder(r.Body).Decode(&body))
	s.False(body["success"].(bool))
	s.Equal("BUSINESS_RULE_VIOLATION", body["error"].(map[string]any)["code"])

	// Confirm no payment was written for household B.
	listR := s.do(http.MethodGet, pathf("/api/payments?household_id=%s", h2ID), nil)
	s.Require().Equal(http.StatusOK, listR.StatusCode)
	var listBody map[string]any
	s.decode(listR, &listBody)
	s.Empty(listBody["data"].([]any), "no payment must be created for a cross-household attempt")
}

// TestPagination_BadInputReturns400 proves that paginationParams (Phase-10 fix F8)
// returns 400 VALIDATION_ERROR on invalid page/per_page values, and that omitting
// the params still defaults gracefully (200 OK).
func (s *E2ESuite) TestPagination_BadInputReturns400() {
	invalid := []string{"?page=0", "?page=-1", "?page=abc", "?per_page=0", "?per_page=999", "?per_page=abc"}
	for _, q := range invalid {
		s.Run(q, func() {
			resp := s.do(http.MethodGet, "/api/households"+q, nil)
			defer resp.Body.Close()
			s.Equal(http.StatusBadRequest, resp.StatusCode)
			var b map[string]any
			s.Require().NoError(json.NewDecoder(resp.Body).Decode(&b))
			s.False(b["success"].(bool))
			s.Equal("VALIDATION_ERROR", b["error"].(map[string]any)["code"])
		})
	}
	// Missing params → default page=1, per_page=20 → 200.
	resp := s.do(http.MethodGet, "/api/households", nil)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)
}

// TestRateLimit_429EnvelopeWithMeta proves that throttled POST /api/pickups
// requests return 429 with error.code = "RATE_LIMITED" and, when OTel tracing
// is wired, meta.trace_id is present. Covers Phase-10 fix F7.
func (s *E2ESuite) TestRateLimit_429EnvelopeWithMeta() {
	// Fire 150 parallel requests — enough to exhaust any burst bucket (CI uses 50,
	// local default is 10). A non-existent household_id is used so requests that
	// do get through return 400 from validation without writing any DB rows.
	const N = 150
	type result struct {
		status int
		body   map[string]any
	}
	results := make([]result, N)
	var wg sync.WaitGroup

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodPost, s.baseURL+"/api/pickups",
				strings.NewReader(`{"household_id":"00000000-0000-0000-0000-000000000001","type":"plastic"}`))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := s.client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			results[idx].status = resp.StatusCode
			if resp.StatusCode == http.StatusTooManyRequests {
				_ = json.NewDecoder(resp.Body).Decode(&results[idx].body)
			}
		}(i)
	}
	wg.Wait()

	var found map[string]any
	for _, r := range results {
		if r.status == http.StatusTooManyRequests {
			found = r.body
			break
		}
	}
	s.Require().NotNil(found, "at least one of %d parallel requests must be rate-limited (429)", N)
	s.False(found["success"].(bool))
	s.Equal("RATE_LIMITED", found["error"].(map[string]any)["code"])
	// meta.trace_id is present when OTel tracing is active.
	if meta, ok := found["meta"].(map[string]any); ok && meta != nil {
		s.NotEmpty(meta["trace_id"])
	}
}

// TestTracing_InboundTraceparentPropagated proves that a W3C traceparent header
// sent by the caller is honoured: the error envelope's meta.trace_id equals the
// trace ID embedded in the inbound header. Covers Phase-10 fix F2.
func (s *E2ESuite) TestTracing_InboundTraceparentPropagated() {
	const traceHex = "0000000000000000deadbeefcafebabe"
	const parentHex = "0102030405060708"
	traceparent := "00-" + traceHex + "-" + parentHex + "-01"

	// Empty body fails validation → 400 with meta.trace_id in the error envelope.
	req, err := http.NewRequest(http.MethodPost, s.baseURL+"/api/households",
		strings.NewReader("{}"))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("traceparent", traceparent)

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)

	var body map[string]any
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))

	meta, ok := body["meta"].(map[string]any)
	if !ok || meta == nil {
		s.T().Skip("meta not present — OTel tracing not configured in this environment")
		return
	}
	gotTraceID, _ := meta["trace_id"].(string)
	s.Require().NotEmpty(gotTraceID)
	s.Equal(traceHex, gotTraceID,
		"error envelope trace_id must match the inbound W3C traceparent trace ID")
}

// TestBodyLimit_413Envelope proves that POSTing a body larger than the 1 MiB JSON
// cap returns 413 with a properly enveloped error (not Echo's bare default JSON).
// Requires the custom echoErrorHandler registered in Phase 11. Covers Phase-10 F7.
func (s *E2ESuite) TestBodyLimit_413Envelope() {
	largeBody := strings.Repeat("x", 1<<20+1) // 1 MiB + 1 byte
	req, err := http.NewRequest(http.MethodPost, s.baseURL+"/api/households",
		strings.NewReader(largeBody))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Require().Equal(http.StatusRequestEntityTooLarge, resp.StatusCode)

	var body map[string]any
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))
	s.False(body["success"].(bool))
	s.Equal("REQUEST_TOO_LARGE", body["error"].(map[string]any)["code"])
}

// TestConfirmPayment_RejectsImageJpg proves that uploading a proof file whose
// part-level Content-Type is "image/jpg" (a non-IANA type removed from the
// allowlist in Phase-10 fix F6) returns 400 VALIDATION_ERROR.
func (s *E2ESuite) TestConfirmPayment_RejectsImageJpg() {
	// Setup: household → pickup → schedule → complete → pending payment.
	var hResp, pResp map[string]any
	r := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "ImageJpg Test Owner", "address": "Jl. ImageJpg No.1",
	})
	s.Require().Equal(http.StatusCreated, r.StatusCode)
	s.decode(r, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)
	defer s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)

	r = s.do(http.MethodPost, "/api/pickups", map[string]any{
		"household_id": householdID, "type": "organic",
	})
	s.Require().Equal(http.StatusCreated, r.StatusCode)
	s.decode(r, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.do(http.MethodPut, pathf("/api/pickups/%s/complete", pickupID), nil)

	var listResp map[string]any
	r = s.do(http.MethodGet, pathf("/api/payments?household_id=%s", householdID), nil)
	s.Require().Equal(http.StatusOK, r.StatusCode)
	s.decode(r, &listResp)
	payments := listResp["data"].([]any)
	s.Require().NotEmpty(payments)
	paymentID := payments[0].(map[string]any)["id"].(string)

	// Build multipart body with the non-IANA "image/jpg" part Content-Type.
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="proof"; filename="receipt.jpg"`)
	hdr.Set("Content-Type", "image/jpg")
	part, err := mw.CreatePart(hdr)
	s.Require().NoError(err)
	_, err = part.Write([]byte("fake-jpg-bytes"))
	s.Require().NoError(err)
	s.Require().NoError(mw.Close())

	req, err := http.NewRequest(http.MethodPut,
		s.baseURL+pathf("/api/payments/%s/confirm", paymentID), buf)
	s.Require().NoError(err)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)

	var respBody map[string]any
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&respBody))
	s.False(respBody["success"].(bool))
	s.Equal("VALIDATION_ERROR", respBody["error"].(map[string]any)["code"])
}
