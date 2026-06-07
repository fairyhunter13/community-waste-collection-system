package handler_test

import (
	"context"
	"encoding/json"
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

func TestHealthCheck_Returns200OK(t *testing.T) {
	_, e := newTestHandler(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
}

func TestVersion_Returns200JSON(t *testing.T) {
	handler.SetVersionInfo("v1.2.3", "abcdef0", "2026-06-07T00:00:00Z")
	t.Cleanup(func() { handler.SetVersionInfo("dev", "unknown", "unknown") })
	_, e := newTestHandler(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/version", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	var out map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Equal(t, "v1.2.3", out["version"])
	require.Equal(t, "abcdef0", out["commit"])
	require.Equal(t, "2026-06-07T00:00:00Z", out["build_date"])
}

func TestReadyCheck_ReturnsUnreadyWhenDBNotConfigured(t *testing.T) {
	_, e := newTestHandler(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"unready"`)
}

func TestServeSwaggerUI_Returns200HTML(t *testing.T) {
	_, e := newTestHandler(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/docs", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "swagger-ui")
	assert.NotContains(t, body, "petstore.swagger.io", "UI must be self-hosted, not redirect to petstore")
}
