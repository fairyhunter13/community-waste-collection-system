// Package handler contains Echo HTTP handlers for all API endpoints.
package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/trace"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/middleware"
	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
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

// echoErrorHandler normalises framework-level errors (e.g. body-limit 413,
// unknown-route 404, method-not-allowed 405) into the same envelope as
// respondError, so every error response carries success=false, an error.code,
// and trace meta regardless of where in the stack the error originates.
func (h *Handler) echoErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}
	code := http.StatusInternalServerError
	errCode := "INTERNAL_ERROR"
	message := "internal server error"

	var he *echo.HTTPError
	if errors.As(err, &he) {
		code = he.Code
		switch code {
		case http.StatusRequestEntityTooLarge:
			errCode = "REQUEST_TOO_LARGE"
			message = "request body exceeds size limit"
		case http.StatusTooManyRequests:
			errCode = "RATE_LIMITED"
			message = "too many requests"
		case http.StatusMethodNotAllowed:
			errCode = "METHOD_NOT_ALLOWED"
			message = "method not allowed"
		case http.StatusNotFound:
			errCode = "NOT_FOUND"
			message = "resource not found"
		default:
			if msg, ok := he.Message.(string); ok && msg != "" {
				message = msg
			}
		}
	}

	meta := extractErrorMeta(c)
	if jsonErr := c.JSON(code, errorResp{
		Success: false,
		Error:   errorBody{Code: errCode, Message: message},
		Meta:    meta,
	}); jsonErr != nil {
		observability.FromContext(c.Request().Context()).ErrorContext(
			c.Request().Context(), "echoErrorHandler: write response failed",
			slog.Any("err", jsonErr),
		)
	}
}

// RegisterRoutes registers all API routes on the given Echo instance.
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.HTTPErrorHandler = h.echoErrorHandler
	e.Use(middleware.RequestID())
	e.Use(echomw.Secure())

	// JSON body cap for non-upload endpoints. The /confirm proof upload sits
	// behind its own http.MaxBytesReader sized to cfg.MaxUploadSizeMB, so the
	// global cap applies to the JSON write surface only.
	jsonBodyLimit := echomw.BodyLimit("1M")

	e.GET("/health", h.HealthCheck)
	e.GET("/readyz", h.ReadyCheck)
	e.GET("/api/version", h.Version)
	e.GET("/api/docs/openapi.yaml", h.ServeOpenAPISpec)
	e.GET("/api/docs", h.ServeSwaggerUI)

	api := e.Group("/api")

	api.POST("/households", h.CreateHousehold, jsonBodyLimit)
	api.GET("/households", h.ListHouseholds)
	api.GET("/households/:id", h.GetHousehold)
	api.DELETE("/households/:id", h.DeleteHousehold)

	pickups := api.Group("/pickups")
	pickups.POST("", h.CreatePickup, middleware.RateLimiter(h.cfg), jsonBodyLimit)
	pickups.GET("", h.ListPickups)
	pickups.PUT("/:id/schedule", h.SchedulePickup, jsonBodyLimit)
	pickups.PUT("/:id/complete", h.CompletePickup)
	pickups.PUT("/:id/cancel", h.CancelPickup)

	api.POST("/payments", h.CreatePayment, jsonBodyLimit)
	api.GET("/payments", h.ListPayments)
	api.PUT("/payments/:id/confirm", h.ConfirmPayment)

	reports := api.Group("/reports")
	reports.GET("/waste-summary", h.WasteSummary)
	reports.GET("/payment-summary", h.PaymentSummary)
	reports.GET("/households/:id/history", h.HouseholdHistory)
}

// paginationParams extracts page and per_page query params with defaults.
// Returns a non-nil error (already written to the response) when a provided
// value is syntactically invalid or out of the accepted range.
func paginationParams(c echo.Context) (page, perPage int, err error) {
	if raw := c.QueryParam("page"); raw != "" {
		v, convErr := strconv.Atoi(raw)
		if convErr != nil || v <= 0 {
			_ = respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "page must be a positive integer")
			return 0, 0, fmt.Errorf("invalid page")
		}
		page = v
	}
	if raw := c.QueryParam("per_page"); raw != "" {
		v, convErr := strconv.Atoi(raw)
		if convErr != nil || v <= 0 || v > 100 {
			_ = respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "per_page must be between 1 and 100")
			return 0, 0, fmt.Errorf("invalid per_page")
		}
		perPage = v
	}
	if page == 0 {
		page = 1
	}
	if perPage == 0 {
		perPage = 20
	}
	return page, perPage, nil
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

type errorMeta struct {
	RequestID string `json:"request_id,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
	SpanID    string `json:"span_id,omitempty"`
}

type errorResp struct {
	Success bool       `json:"success"`
	Error   errorBody  `json:"error"`
	Meta    *errorMeta `json:"meta,omitempty"`
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
	meta := extractErrorMeta(c)
	return c.JSON(status, errorResp{
		Success: false,
		Error:   errorBody{Code: code, Message: message},
		Meta:    meta,
	})
}

func extractErrorMeta(c echo.Context) *errorMeta {
	span := trace.SpanFromContext(c.Request().Context())
	sc := span.SpanContext()
	if !sc.IsValid() {
		return nil
	}
	m := &errorMeta{
		TraceID: sc.TraceID().String(),
		SpanID:  sc.SpanID().String(),
	}
	if rid := c.Response().Header().Get("X-Request-Id"); rid != "" {
		m.RequestID = rid
	}
	return m
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
		observability.FromContext(c.Request().Context()).ErrorContext(c.Request().Context(),
			"unmapped error → 500",
			slog.String("op", "handler.mapError"),
			slog.Any("err", err),
		)
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
