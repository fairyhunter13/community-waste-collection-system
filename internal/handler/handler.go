// Package handler contains Echo HTTP handlers for all API endpoints.
package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/middleware"
)

func newValidator(db *sqlx.DB) *validator.Validate {
	v := validator.New()
	_ = v.RegisterValidation("positive_decimal", func(fl validator.FieldLevel) bool {
		d, ok := fl.Field().Interface().(decimal.Decimal)
		return ok && d.IsPositive()
	})
	// Always register DB validators so struct tag parsing succeeds.
	// When db is nil (unit tests), they pass through without querying.
	_ = v.RegisterValidationCtx("db_exists_household", func(ctx context.Context, fl validator.FieldLevel) bool {
		if db == nil {
			return true
		}
		id, ok := fl.Field().Interface().(uuid.UUID)
		if !ok || id == uuid.Nil {
			return false
		}
		var n int
		_ = db.GetContext(ctx, &n, "SELECT COUNT(*) FROM households WHERE id=$1", id)
		return n > 0
	})
	_ = v.RegisterValidationCtx("db_exists_pickup", func(ctx context.Context, fl validator.FieldLevel) bool {
		if db == nil {
			return true
		}
		id, ok := fl.Field().Interface().(uuid.UUID)
		if !ok || id == uuid.Nil {
			return false
		}
		var n int
		_ = db.GetContext(ctx, &n, "SELECT COUNT(*) FROM waste_pickups WHERE id=$1", id)
		return n > 0
	})
	return v
}

// Handler holds all service dependencies and shared helpers for HTTP handlers.
type Handler struct {
	householdSvc domain.HouseholdService
	pickupSvc    domain.PickupService
	paymentSvc   domain.PaymentService
	reportSvc    domain.ReportService
	validate     *validator.Validate
	cfg          *config.Config
	db           *sqlx.DB
}

// New creates a new Handler with all service dependencies wired.
func New(
	hSvc domain.HouseholdService,
	pSvc domain.PickupService,
	paymentSvc domain.PaymentService,
	rSvc domain.ReportService,
	cfg *config.Config,
	db *sqlx.DB,
) *Handler {
	return &Handler{
		householdSvc: hSvc,
		pickupSvc:    pSvc,
		paymentSvc:   paymentSvc,
		reportSvc:    rSvc,
		validate:     newValidator(db),
		cfg:          cfg,
		db:           db,
	}
}

// RegisterRoutes registers all API routes on the given Echo instance.
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/health", h.HealthCheck)
	e.GET("/api/docs/openapi.yaml", h.ServeOpenAPISpec)
	e.GET("/api/docs", h.ServeSwaggerUI)

	api := e.Group("/api")

	api.POST("/households", h.CreateHousehold)
	api.GET("/households", h.ListHouseholds)
	api.GET("/households/:id", h.GetHousehold)
	api.DELETE("/households/:id", h.DeleteHousehold)

	pickups := api.Group("/pickups")
	pickups.POST("", h.CreatePickup, middleware.RateLimiter(h.cfg))
	pickups.GET("", h.ListPickups)
	pickups.PUT("/:id/schedule", h.SchedulePickup)
	pickups.PUT("/:id/complete", h.CompletePickup)
	pickups.PUT("/:id/cancel", h.CancelPickup)

	api.POST("/payments", h.CreatePayment)
	api.GET("/payments", h.ListPayments)
	api.PUT("/payments/:id/confirm", h.ConfirmPayment)

	reports := api.Group("/reports")
	reports.GET("/waste-summary", h.WasteSummary)
	reports.GET("/payment-summary", h.PaymentSummary)
	reports.GET("/households/:id/history", h.HouseholdHistory)
}

// paginationParams extracts page and per_page query params with defaults.
func paginationParams(c echo.Context) (page, perPage int) {
	page, _ = strconv.Atoi(c.QueryParam("page"))
	perPage, _ = strconv.Atoi(c.QueryParam("per_page"))
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	return page, perPage
}

type successResp struct {
	Success bool `json:"success"`
	Data    any  `json:"data"`
}

type listMeta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type listResp struct {
	Success bool     `json:"success"`
	Data    any      `json:"data"`
	Meta    listMeta `json:"meta"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResp struct {
	Success bool      `json:"success"`
	Error   errorBody `json:"error"`
}

func respond(c echo.Context, status int, data any) error {
	return c.JSON(status, successResp{Success: true, Data: data})
}

func respondList(c echo.Context, data any, total, page, perPage int) error {
	totalPages := (total + perPage - 1) / perPage
	if totalPages == 0 {
		totalPages = 1
	}
	return c.JSON(http.StatusOK, listResp{
		Success: true,
		Data:    data,
		Meta:    listMeta{Page: page, PerPage: perPage, Total: total, TotalPages: totalPages},
	})
}

func respondError(c echo.Context, status int, code, message string) error {
	return c.JSON(status, errorResp{
		Success: false,
		Error:   errorBody{Code: code, Message: message},
	})
}

func mapError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return respondError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrConflict):
		return respondError(c, http.StatusConflict, "CONFLICT", err.Error())
	case errors.Is(err, domain.ErrBusinessRule):
		return respondError(c, http.StatusUnprocessableEntity, "BUSINESS_RULE_VIOLATION", err.Error())
	case errors.Is(err, domain.ErrValidation):
		return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	default:
		return respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}

func bindAndValidate(c echo.Context, v *validator.Validate, dst any) error {
	if err := c.Bind(dst); err != nil {
		_ = respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return err
	}
	if err := v.StructCtx(c.Request().Context(), dst); err != nil {
		_ = respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return err
	}
	return nil
}
