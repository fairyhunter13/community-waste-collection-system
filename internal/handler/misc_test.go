package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
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
	h := handler.New(hSvc, pSvc, paySvc, rptSvc, config.Load(), nil, nil)
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

// TestEchoErrorHandler_404_UnknownRoute verifies the custom error handler emits
// the standard envelope with code NOT_FOUND for unregistered paths.
func TestEchoErrorHandler_404_UnknownRoute(t *testing.T) {
	_, e := newTestHandler(t)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/this-route-does-not-exist", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp["success"].(bool))
	errObj, ok := resp["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "NOT_FOUND", errObj["code"])
}

// TestEchoErrorHandler_405_MethodNotAllowed verifies the standard envelope for
// a method that is not registered on an existing path.
func TestEchoErrorHandler_405_MethodNotAllowed(t *testing.T) {
	_, e := newTestHandler(t)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp["success"].(bool))
	errObj, ok := resp["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "METHOD_NOT_ALLOWED", errObj["code"])
}

// TestPaginationParams_ZeroPage verifies that page=0 is rejected with 400.
func TestPaginationParams_ZeroPage(t *testing.T) {
	h := mocks.NewHouseholdService(t)
	_, e := newTestHandler(t)
	_ = h

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/households?page=0", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestPaginationParams_NegativePage verifies that page=-5 is rejected with 400.
func TestPaginationParams_NegativePage(t *testing.T) {
	_, e := newTestHandler(t)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/households?page=-5", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestPaginationParams_NonIntPage verifies that page=abc is rejected with 400.
func TestPaginationParams_NonIntPage(t *testing.T) {
	_, e := newTestHandler(t)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/households?page=abc", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestPaginationParams_PerPageTooLarge verifies that per_page>100 is rejected with 400.
func TestPaginationParams_PerPageTooLarge(t *testing.T) {
	_, e := newTestHandler(t)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/households?per_page=200", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestPaginationParams_NonIntPerPage verifies that per_page=abc is rejected with 400.
func TestPaginationParams_NonIntPerPage(t *testing.T) {
	_, e := newTestHandler(t)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/households?per_page=abc", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestPaginationParams_ZeroPerPage verifies that per_page=0 is rejected with 400.
func TestPaginationParams_ZeroPerPage(t *testing.T) {
	_, e := newTestHandler(t)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/households?per_page=0", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// U4: /readyz returns 503 when the DB is configured but PingContext fails.
func TestReadyCheck_ReturnsUnreadyWhenDBPingFails(t *testing.T) {
	// Open a handle pointing at a port nothing is listening on. sqlx.Open does
	// not dial immediately, so this succeeds; PingContext will fail later.
	db, err := sqlx.Open("postgres",
		"host=127.0.0.1 port=1 dbname=fake user=fake password=fake sslmode=disable connect_timeout=1",
	)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	hSvc := mocks.NewHouseholdService(t)
	pSvc := mocks.NewPickupService(t)
	paySvc := mocks.NewPaymentService(t)
	rptSvc := mocks.NewReportService(t)
	h := handler.New(hSvc, pSvc, paySvc, rptSvc, config.Load(), db, nil)
	e := echo.New()
	h.RegisterRoutes(e)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"unready"`)
	assert.Contains(t, rec.Body.String(), `"db":"unreachable"`)
}
