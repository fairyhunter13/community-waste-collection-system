package handler_test

import (
	"context"
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

// U4: /readyz returns 503 when the DB is configured but PingContext fails.
func TestReadyCheck_ReturnsUnreadyWhenDBPingFails(t *testing.T) {
	// Open a handle pointing at a port nothing is listening on. sqlx.Open does
	// not dial immediately, so this succeeds; PingContext will fail later.
	db, err := sqlx.Open("postgres",
		"host=127.0.0.1 port=1 dbname=fake user=fake password=fake sslmode=disable connect_timeout=1",
	)
	require.NoError(t, err)
	defer db.Close()

	hSvc := mocks.NewHouseholdService(t)
	pSvc := mocks.NewPickupService(t)
	paySvc := mocks.NewPaymentService(t)
	rptSvc := mocks.NewReportService(t)
	h := handler.New(hSvc, pSvc, paySvc, rptSvc, config.Load(), db)
	e := echo.New()
	h.RegisterRoutes(e)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"unready"`)
	assert.Contains(t, rec.Body.String(), `"db":"unreachable"`)
}
