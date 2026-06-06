//go:build e2e

package e2e_test

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// TestConcurrent_Schedule_OnlyOneSucceeds proves the T24 race fix:
// firing N parallel Schedule requests at the same pending pickup yields
// exactly one 200 (or 2xx) success and the rest 409 — the conditional
// UPDATE in the repository now refuses to overwrite a non-pending row.
func (s *E2ESuite) TestConcurrent_Schedule_OnlyOneSucceeds() {
	const N = 8

	// Setup: household + pending pickup.
	var hResp, pResp map[string]any
	r := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Race Schedule Owner",
		"address":    "Jl. Race No. 1",
	})
	s.Require().Equal(http.StatusCreated, r.StatusCode)
	s.decode(r, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)
	defer s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)

	r = s.do(http.MethodPost, "/api/pickups", map[string]any{
		"household_id": householdID,
		"type":         "paper",
	})
	s.Require().Equal(http.StatusCreated, r.StatusCode)
	s.decode(r, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	// Fire N parallel Schedule requests against the same pickup.
	var success, conflict int64
	var wg sync.WaitGroup
	startGate := make(chan struct{})
	pickupDate := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startGate
			resp := s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
				"pickup_date": pickupDate,
			})
			resp.Body.Close()
			switch resp.StatusCode {
			case http.StatusOK:
				atomic.AddInt64(&success, 1)
			case http.StatusConflict:
				atomic.AddInt64(&conflict, 1)
			}
		}()
	}
	close(startGate)
	wg.Wait()

	s.Equal(int64(1), atomic.LoadInt64(&success),
		"exactly one Schedule must succeed under N=%d parallel race", N)
	s.Equal(int64(N-1), atomic.LoadInt64(&conflict),
		"the other N-1 callers must each get 409 Conflict")
}

// TestConcurrent_Complete_OnlyOneSucceeds proves the T25 race fix:
// firing N parallel Complete requests on the same scheduled pickup
// yields exactly one 200 and N-1 409s, AND exactly one payment row
// is created (the conditional UPDATE in UpdateStatus aborts the tx
// for the losers; partial UNIQUE index from T27 is the safety net).
func (s *E2ESuite) TestConcurrent_Complete_OnlyOneSucceeds() {
	const N = 8

	// Setup: household + pickup → schedule it.
	var hResp, pResp map[string]any
	r := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Race Complete Owner",
		"address":    "Jl. Race C No. 1",
	})
	s.Require().Equal(http.StatusCreated, r.StatusCode)
	s.decode(r, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)
	defer s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)

	r = s.do(http.MethodPost, "/api/pickups", map[string]any{
		"household_id": householdID,
		"type":         "organic",
	})
	s.Require().Equal(http.StatusCreated, r.StatusCode)
	s.decode(r, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	r = s.do(http.MethodPut, pathf("/api/pickups/%s/schedule", pickupID), map[string]any{
		"pickup_date": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
	})
	s.Require().Equal(http.StatusOK, r.StatusCode)
	r.Body.Close()

	// Fire N parallel Complete requests against the same pickup.
	var success, conflict int64
	var wg sync.WaitGroup
	startGate := make(chan struct{})

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startGate
			resp := s.do(http.MethodPut, pathf("/api/pickups/%s/complete", pickupID), nil)
			resp.Body.Close()
			switch resp.StatusCode {
			case http.StatusOK:
				atomic.AddInt64(&success, 1)
			case http.StatusConflict:
				atomic.AddInt64(&conflict, 1)
			}
		}()
	}
	close(startGate)
	wg.Wait()

	s.Equal(int64(1), atomic.LoadInt64(&success),
		"exactly one Complete must succeed under N=%d parallel race", N)
	s.Equal(int64(N-1), atomic.LoadInt64(&conflict),
		"the other N-1 callers must each get 409 Conflict")

	// Exactly ONE payment must exist for this pickup.
	r = s.do(http.MethodGet, pathf("/api/payments?household_id=%s", householdID), nil)
	s.Require().Equal(http.StatusOK, r.StatusCode)
	var listResp map[string]any
	s.decode(r, &listResp)
	payments := listResp["data"].([]any)
	s.Len(payments, 1, "exactly one payment must be created despite N parallel Complete calls")
}
