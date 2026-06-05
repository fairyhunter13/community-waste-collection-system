package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
)

// CreateHousehold handles POST /api/households.
func (h *Handler) CreateHousehold(c echo.Context) error {
	var req domain.CreateHouseholdRequest
	if err := bindAndValidate(c, h.validate, &req); err != nil {
		return err
	}
	household, err := h.householdSvc.Create(c.Request().Context(), req)
	if err != nil {
		return mapError(c, err)
	}
	return respond(c, http.StatusCreated, household)
}

// ListHouseholds handles GET /api/households.
func (h *Handler) ListHouseholds(c echo.Context) error {
	page, perPage := paginationParams(c)
	households, total, err := h.householdSvc.List(c.Request().Context(), page, perPage)
	if err != nil {
		return mapError(c, err)
	}
	return respondList(c, households, total, page, perPage)
}

// GetHousehold handles GET /api/households/:id.
func (h *Handler) GetHousehold(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid household id")
	}
	household, err := h.householdSvc.GetByID(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err)
	}
	return respond(c, http.StatusOK, household)
}

// DeleteHousehold handles DELETE /api/households/:id.
func (h *Handler) DeleteHousehold(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid household id")
	}
	if err := h.householdSvc.Delete(c.Request().Context(), id); err != nil {
		return mapError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
