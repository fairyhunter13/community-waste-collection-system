package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
)

// CreateHousehold handles POST /api/households.
func (h *Handler) CreateHousehold(c echo.Context) error {
	var req domain.CreateHouseholdRequest
	if err := bindAndValidate(c, h.validate, &req); err != nil {
		return err
	}
	trace.SpanFromContext(c.Request().Context()).SetAttributes(
		attribute.String("household.owner_name", req.OwnerName),
	)
	household, err := h.householdSvc.Create(c.Request().Context(), req)
	if err != nil {
		return mapError(c, err)
	}
	return respond(c, http.StatusCreated, household)
}

// ListHouseholds handles GET /api/households.
func (h *Handler) ListHouseholds(c echo.Context) error {
	page, perPage, err := paginationParams(c)
	if err != nil {
		return err
	}
	trace.SpanFromContext(c.Request().Context()).SetAttributes(
		attribute.Int("pagination.page", page),
		attribute.Int("pagination.per_page", perPage),
	)
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
	trace.SpanFromContext(c.Request().Context()).SetAttributes(
		attribute.String("household.id", id.String()),
	)
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
	trace.SpanFromContext(c.Request().Context()).SetAttributes(
		attribute.String("household.id", id.String()),
	)
	if err := h.householdSvc.Delete(c.Request().Context(), id); err != nil {
		return mapError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
