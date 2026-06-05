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
