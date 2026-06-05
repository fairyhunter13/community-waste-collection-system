//go:build e2e

package e2e_test

import (
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
