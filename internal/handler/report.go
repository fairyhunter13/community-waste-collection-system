package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// WasteSummary handles GET /api/reports/waste-summary.
func (h *Handler) WasteSummary(c echo.Context) error {
	summary, err := h.reportSvc.WasteSummary(c.Request().Context())
	if err != nil {
		return mapError(c, err)
	}
	return respond(c, http.StatusOK, map[string]any{"by_type": summary})
}

// PaymentSummary handles GET /api/reports/payment-summary.
func (h *Handler) PaymentSummary(c echo.Context) error {
	summary, err := h.reportSvc.PaymentSummary(c.Request().Context())
	if err != nil {
		return mapError(c, err)
	}
	return respond(c, http.StatusOK, summary)
}

// HouseholdHistory handles GET /api/reports/households/:id/history.
func (h *Handler) HouseholdHistory(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid household id")
	}
	trace.SpanFromContext(c.Request().Context()).SetAttributes(
		attribute.String("household.id", id.String()),
	)
	history, err := h.reportSvc.HouseholdHistory(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err)
	}
	return respond(c, http.StatusOK, history)
}
