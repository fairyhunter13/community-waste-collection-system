package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/handler"
	"github.com/fairyhunter13/community-waste-collection-system/internal/mocks"
)

func newTestHandler(t *testing.T) (*handler.Handler, *echo.Echo) {
	t.Helper()
	hSvc := mocks.NewHouseholdService(t)
	pSvc := mocks.NewPickupService(t)
	paySvc := mocks.NewPaymentService(t)
	rptSvc := mocks.NewReportService(t)
	h := handler.New(hSvc, pSvc, paySvc, rptSvc, config.Load(), nil)
	e := echo.New()
	h.RegisterRoutes(e)
	return h, e
}

func TestHealthCheck_Returns200OK(t *testing.T) {
	_, e := newTestHandler(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
}

func TestReadyCheck_ReturnsUnreadyWhenDBNotConfigured(t *testing.T) {
	_, e := newTestHandler(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"unready"`)
}
