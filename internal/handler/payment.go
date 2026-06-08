package handler

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

// allowedProofMIME is the closed allowlist for proof upload Content-Type values
// AND for the magic-byte sniff. Both the declared type AND the sniffed type
// must appear in this set; a mismatch returns 400.
var allowedProofMIME = map[string]bool{
	"image/jpeg":      true,
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
	page, perPage, err := paginationParams(c)
	if err != nil {
		return err
	}
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
// It enforces two content-type guards: the declared Content-Type part header must
// be in the allowlist, AND the magic bytes of the file must match the declared type
// (prevents a client lying about its content type to bypass the filter).
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
		observability.FromContext(c.Request().Context()).WarnContext(c.Request().Context(),
			"proof file upload rejected",
			slog.String("op", "ConfirmPayment.upload"),
			slog.Any("err", err),
		)
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return respondError(c, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE",
				"proof file exceeds upload size limit")
		}
		return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR",
			"proof file required")
	}
	defer func() { _ = file.Close() }()

	// Gate 1: declared Content-Type must be in the allowlist.
	declaredType := header.Header.Get("Content-Type")
	if !allowedProofMIME[declaredType] {
		return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR",
			"unsupported proof type: must be image/jpeg, image/png, or application/pdf")
	}

	// Gate 2: magic-byte sniff — read first 512 bytes, detect the actual type,
	// and reject if it does not match the declared type. io.MultiReader rewinds
	// so the full file body is still available for the service upload call.
	sniffBuf := make([]byte, 512)
	n, err := io.ReadFull(file, sniffBuf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "could not read proof file")
	}
	sniffedType := http.DetectContentType(sniffBuf[:n])
	if !allowedProofMIME[sniffedType] {
		return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR",
			"proof file content does not match a supported type")
	}

	// Rewind: combine the already-read prefix with the remainder.
	fullReader := io.MultiReader(bytes.NewReader(sniffBuf[:n]), file)

	payment, err := h.paymentSvc.Confirm(c.Request().Context(), id, fullReader, header.Size, declaredType)
	if err != nil {
		return mapError(c, err)
	}
	return respond(c, http.StatusOK, payment)
}
