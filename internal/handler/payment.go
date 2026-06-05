package handler

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
)

// CreatePayment handles POST /api/payments.
func (h *Handler) CreatePayment(c echo.Context) error {
	var req domain.CreatePaymentRequest
	if err := bindAndValidate(c, h.validate, &req); err != nil {
		return err
	}
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

	if hid := c.QueryParam("household_id"); hid != "" {
		id, err := uuid.Parse(hid)
		if err != nil {
			return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid household_id")
		}
		filter.HouseholdID = &id
	}
	if s := c.QueryParam("status"); s != "" {
		status := domain.PaymentStatus(s)
		filter.Status = &status
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

	maxBytes := int64(h.cfg.MaxUploadSizeMB) << 20
	c.Request().Body = http.MaxBytesReader(c.Response().Writer, c.Request().Body, maxBytes)

	file, header, err := c.Request().FormFile("proof")
	if err != nil {
		return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR",
			fmt.Sprintf("proof file required: %v", err))
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	payment, err := h.paymentSvc.Confirm(c.Request().Context(), id, file, header.Size, contentType)
	if err != nil {
		return mapError(c, err)
	}
	return respond(c, http.StatusOK, payment)
}
