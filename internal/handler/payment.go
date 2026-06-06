package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
)

// allowedProofMIME is the closed allowlist of Content-Type values the proof
// upload endpoint will accept. Any value outside this set returns 400 — we do
// not attempt content sniffing or normalisation because the wire-level type
// is the contract clients agree to.
var allowedProofMIME = map[string]bool{
	"image/jpeg":      true,
	"image/jpg":       true,
	"image/png":       true,
	"application/pdf": true,
}

// CreatePayment handles POST /api/payments.
func (h *Handler) CreatePayment(c echo.Context) error {
	var req domain.CreatePaymentRequest
	if err := bindAndValidate(c, h.validate, &req); err != nil {
		return err
	}
	trace.SpanFromContext(c.Request().Context()).SetAttributes(
		attribute.String("payment.household_id", req.HouseholdID.String()),
		attribute.String("payment.waste_id", req.WasteID.String()),
	)
	payment, err := h.paymentSvc.Create(c.Request().Context(), req)
	if err != nil {
		return mapError(c, err)
	}
	return respond(c, http.StatusCreated, payment)
}

// ListPayments handles GET /api/payments with optional filters.
func (h *Handler) ListPayments(c echo.Context) error {
	page, perPage := paginationParams(c)
	filter := domain.PaymentFilter{Page: page, PerPage: perPage}

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
		status := domain.PaymentStatus(s)
		switch status {
		case domain.PaymentStatusPending, domain.PaymentStatusPaid, domain.PaymentStatusFailed:
			filter.Status = &status
			span.SetAttributes(attribute.String("filter.status", s))
		default:
			return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR",
				"invalid status: must be pending, paid, or failed")
		}
	}
	if df := c.QueryParam("date_from"); df != "" {
		t, err := time.Parse(time.RFC3339, df)
		if err != nil {
			return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid date_from: must be RFC3339")
		}
		filter.DateFrom = &t
	}
	if dt := c.QueryParam("date_to"); dt != "" {
		t, err := time.Parse(time.RFC3339, dt)
		if err != nil {
			return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid date_to: must be RFC3339")
		}
		filter.DateTo = &t
	}

	payments, total, err := h.paymentSvc.List(c.Request().Context(), filter)
	if err != nil {
		return mapError(c, err)
	}
	return respondList(c, payments, total, page, perPage)
}

// ConfirmPayment handles PUT /api/payments/:id/confirm with multipart file upload.
func (h *Handler) ConfirmPayment(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid payment id")
	}
	trace.SpanFromContext(c.Request().Context()).SetAttributes(
		attribute.String("payment.id", id.String()),
	)

	maxBytes := int64(h.cfg.MaxUploadSizeMB) << 20
	c.Request().Body = http.MaxBytesReader(c.Response().Writer, c.Request().Body, maxBytes)

	file, header, err := c.Request().FormFile("proof")
	if err != nil {
		slog.WarnContext(c.Request().Context(), "proof file upload rejected", "error", err.Error())
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return respondError(c, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE",
				"proof file exceeds upload size limit")
		}
		return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR",
			"proof file required")
	}
	defer func() { _ = file.Close() }()

	contentType := header.Header.Get("Content-Type")
	if !allowedProofMIME[contentType] {
		return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR",
			"unsupported proof type: must be image/jpeg, image/png, or application/pdf")
	}

	payment, err := h.paymentSvc.Confirm(c.Request().Context(), id, file, header.Size, contentType)
	if err != nil {
		return mapError(c, err)
	}
	return respond(c, http.StatusOK, payment)
}
