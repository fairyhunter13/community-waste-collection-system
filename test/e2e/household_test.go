//go:build e2e

package e2e_test

import (
	"fmt"
	"net/http"
)

func (s *E2ESuite) TestHousehold_CRUD() {
	// Create
	var created map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "E2E Owner",
		"address":    "Jl. E2E No. 1",
	})
	s.Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &created)
	s.True(created["success"].(bool))

	data := created["data"].(map[string]any)
	id := data["id"].(string)
	s.NotEmpty(id)

	// Get
	resp = s.do(http.MethodGet, pathf("/api/households/%s", id), nil)
	s.Equal(http.StatusOK, resp.StatusCode)
	var got map[string]any
	s.decode(resp, &got)
	s.Equal(id, got["data"].(map[string]any)["id"])

	// List
	resp = s.do(http.MethodGet, "/api/households", nil)
	s.Equal(http.StatusOK, resp.StatusCode)
	var list map[string]any
	s.decode(resp, &list)
	s.NotNil(list["meta"])

	// Delete
	resp = s.do(http.MethodDelete, pathf("/api/households/%s", id), nil)
	s.Equal(http.StatusNoContent, resp.StatusCode)

	// Get 404
	resp = s.do(http.MethodGet, pathf("/api/households/%s", id), nil)
	s.Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *E2ESuite) TestCreateHousehold_400_MissingFields() {
	resp := s.do(http.MethodPost, "/api/households", map[string]any{})
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

// TestHousehold_DeleteCascades_PickupsAndPayments verifies that deleting a
// household also removes its associated pickups and payments via DB cascade.
func (s *E2ESuite) TestHousehold_DeleteCascades_PickupsAndPayments() {
	var hResp, pResp map[string]any
	resp := s.do(http.MethodPost, "/api/households", map[string]any{
		"owner_name": "Cascade Owner",
		"address":    "Jl. Cascade No. 1",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &hResp)
	householdID := hResp["data"].(map[string]any)["id"].(string)

	resp = s.do(http.MethodPost, "/api/pickups", map[string]any{"household_id": householdID, "type": "plastic"})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &pResp)
	pickupID := pResp["data"].(map[string]any)["id"].(string)

	// Direct payment creation.
	var payResp map[string]any
	resp = s.do(http.MethodPost, "/api/payments", map[string]any{
		"household_id": householdID,
		"waste_id":     pickupID,
		"amount":       "50000.00",
	})
	s.Require().Equal(http.StatusCreated, resp.StatusCode)
	s.decode(resp, &payResp)

	// Delete the household.
	resp = s.do(http.MethodDelete, pathf("/api/households/%s", householdID), nil)
	s.Require().Equal(http.StatusNoContent, resp.StatusCode)

	// Household is gone.
	resp = s.do(http.MethodGet, pathf("/api/households/%s", householdID), nil)
	s.Equal(http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()

	// Pickups for this household must be empty (cascaded).
	resp = s.do(http.MethodGet, fmt.Sprintf("/api/pickups?household_id=%s", householdID), nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	var pickupList map[string]any
	s.decode(resp, &pickupList)
	s.Empty(pickupList["data"].([]any), "pickups must be deleted with household")

	// Payments for this household must be empty (cascaded).
	resp = s.do(http.MethodGet, fmt.Sprintf("/api/payments?household_id=%s", householdID), nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	var payList map[string]any
	s.decode(resp, &payList)
	s.Empty(payList["data"].([]any), "payments must be deleted with household")
}

// TestHousehold_Pagination_EmptyPage requests a page beyond the last one and
// expects a 200 with an empty data array rather than an error.
func (s *E2ESuite) TestHousehold_Pagination_EmptyPage() {
	// Request page 999 with a per_page that cannot possibly have data.
	resp := s.do(http.MethodGet, "/api/households?page=999&per_page=20", nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	var listResp map[string]any
	s.decode(resp, &listResp)
	s.True(listResp["success"].(bool))
	data := listResp["data"].([]any)
	s.Empty(data, "requesting a non-existent page must return empty data")
}

func (s *E2ESuite) TestHousehold_Pagination() {
	// Create 25 households, track IDs for cleanup
	ids := make([]string, 25)
	for i := range 25 {
		var resp map[string]any
		r := s.do(http.MethodPost, "/api/households", map[string]any{
			"owner_name": fmt.Sprintf("Pagination Owner %02d", i+1),
			"address":    fmt.Sprintf("Jl. Pagination No. %d", i+1),
		})
		s.Require().Equal(http.StatusCreated, r.StatusCode)
		s.decode(r, &resp)
		ids[i] = resp["data"].(map[string]any)["id"].(string)
	}
	defer func() {
		for _, id := range ids {
			s.do(http.MethodDelete, pathf("/api/households/%s", id), nil)
		}
	}()

	// Page 1: expect 20 results, total ≥ 25, total_pages ≥ 2
	r := s.do(http.MethodGet, "/api/households?per_page=20&page=1", nil)
	s.Require().Equal(http.StatusOK, r.StatusCode)
	var page1 map[string]any
	s.decode(r, &page1)
	data1 := page1["data"].([]any)
	meta := page1["meta"].(map[string]any)
	s.Equal(20, len(data1))
	s.GreaterOrEqual(int(meta["total"].(float64)), 25)
	s.GreaterOrEqual(int(meta["total_pages"].(float64)), 2)

	// Page 2: must have at least 1 result
	r = s.do(http.MethodGet, "/api/households?per_page=20&page=2", nil)
	s.Require().Equal(http.StatusOK, r.StatusCode)
	var page2 map[string]any
	s.decode(r, &page2)
	data2 := page2["data"].([]any)
	s.GreaterOrEqual(len(data2), 1)
}
