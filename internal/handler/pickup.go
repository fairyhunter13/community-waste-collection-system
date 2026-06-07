package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
)

// CreatePickup handles POST /api/pickups (rate-limited).
func (h *Handler) CreatePickup(c echo.Context) error {
	var req domain.CreatePickupRequest
	if err := bindAndValidate(c, h.validate, &req); err != nil {
		return err
	}
	trace.SpanFromContext(c.Request().Context()).SetAttributes(
		attribute.String("pickup.type", string(req.Type)),
		attribute.String("pickup.household_id", req.HouseholdID.String()),
	)
	pickup, err := h.pickupSvc.Create(c.Request().Context(), req)
	if err != nil {
		return mapError(c, err)
	}
	return respond(c, http.StatusCreated, pickup)
}

// ListPickups handles GET /api/pickups with optional status and household_id filters.
func (h *Handler) ListPickups(c echo.Context) error {
	page, perPage, err := paginationParams(c)
	if err != nil {
		return err
	}
	filter := domain.PickupFilter{Page: page, PerPage: perPage}

	span := trace.SpanFromContext(c.Request().Context())
	span.SetAttributes(
		attribute.Int("pagination.page", page),
		attribute.Int("pagination.per_page", perPage),
	)

	if hid := c.QueryParam("household_id"); hid != "" {
		id, err := uuid.Parse(hid)
		if err != nil {
			return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid household_id")
		}
		filter.HouseholdID = &id
		span.SetAttributes(attribute.String("filter.household_id", id.String()))
	}
	if s := c.QueryParam("status"); s != "" {
		status := domain.PickupStatus(s)
		switch status {
		case domain.PickupStatusPending, domain.PickupStatusScheduled,
			domain.PickupStatusCompleted, domain.PickupStatusCanceled:
			filter.Status = &status
			span.SetAttributes(attribute.String("filter.status", s))
		default:
			return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR",
				"invalid status: must be pending, scheduled, completed, or canceled")
		}
	}

	pickups, total, err := h.pickupSvc.List(c.Request().Context(), filter)
	if err != nil {
		return mapError(c, err)
	}
	return respondList(c, pickups, total, page, perPage)
}

// SchedulePickup handles PUT /api/pickups/:id/schedule.
func (h *Handler) SchedulePickup(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid pickup id")
	}
	var req domain.SchedulePickupRequest
	if err := bindAndValidate(c, h.validate, &req); err != nil {
		return err
	}
	trace.SpanFromContext(c.Request().Context()).SetAttributes(
		attribute.String("pickup.id", id.String()),
		attribute.String("pickup.scheduled_date", req.PickupDate.String()),
	)
	pickup, err := h.pickupSvc.Schedule(c.Request().Context(), id, req)
	if err != nil {
		return mapError(c, err)
	}
	return respond(c, http.StatusOK, pickup)
}

// CompletePickup handles PUT /api/pickups/:id/complete.
func (h *Handler) CompletePickup(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid pickup id")
	}
	trace.SpanFromContext(c.Request().Context()).SetAttributes(
		attribute.String("pickup.id", id.String()),
	)
	pickup, err := h.pickupSvc.Complete(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err)
	}
	return respond(c, http.StatusOK, pickup)
}

// CancelPickup handles PUT /api/pickups/:id/cancel.
func (h *Handler) CancelPickup(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid pickup id")
	}
	trace.SpanFromContext(c.Request().Context()).SetAttributes(
		attribute.String("pickup.id", id.String()),
	)
	pickup, err := h.pickupSvc.Cancel(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err)
	}
	return respond(c, http.StatusOK, pickup)
}
