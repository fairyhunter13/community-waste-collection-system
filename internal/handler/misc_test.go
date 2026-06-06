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

func TestServeOpenAPISpec_Returns200YAML(t *testing.T) {
	_, e := newTestHandler(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/docs/openapi.yaml", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/yaml; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.NotEmpty(t, rec.Body.Bytes())
}

func TestServeSwaggerUI_Returns200HTML(t *testing.T) {
	_, e := newTestHandler(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/docs", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "swagger")
}
