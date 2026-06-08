package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
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

func newTestHandlerWithDB(t *testing.T, db *sqlx.DB) (*handler.Handler, *echo.Echo) {
	t.Helper()
	hSvc := mocks.NewHouseholdService(t)
	pSvc := mocks.NewPickupService(t)
	paySvc := mocks.NewPaymentService(t)
	rptSvc := mocks.NewReportService(t)
	h := handler.New(hSvc, pSvc, paySvc, rptSvc, config.Load(), db, nil)
	e := echo.New()
	h.RegisterRoutes(e)
	return h, e
}

// TestEchoErrorHandler_413_BodyTooLarge verifies the REQUEST_TOO_LARGE envelope.
func TestEchoErrorHandler_413_BodyTooLarge(t *testing.T) {
	_, e := newTestHandler(t)

	// Send a body just over the 1 M JSON cap registered on POST /api/households.
	body := strings.Repeat("x", (1<<20)+1)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/households",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	errObj, ok := resp["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "REQUEST_TOO_LARGE", errObj["code"])
}

// TestNewValidator_PositiveDecimal_RejectsZeroAmount verifies that amount=0
// fails the positive_decimal custom validator and returns 400.
func TestNewValidator_PositiveDecimal_RejectsZeroAmount(t *testing.T) {
	_, e := newTestHandler(t)

	householdID := uuid.New()
	wasteID := uuid.New()
	body := fmt.Sprintf(`{"household_id":%q,"waste_id":%q,"amount":"0"}`,
		householdID, wasteID)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/payments",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp["success"].(bool))
}

// TestNewValidator_DBExistsHousehold_ReturnsFalseWhenNotFound verifies that
// db_exists_household rejects a UUID absent from the database.
func TestNewValidator_DBExistsHousehold_ReturnsFalseWhenNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	sqlxDB := sqlx.NewDb(db, "postgres")

	_, e := newTestHandlerWithDB(t, sqlxDB)

	householdID := uuid.New()
	wasteID := uuid.New()

	// households lookup returns 0 rows → db_exists_household fails
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM households`).
		WithArgs(householdID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	body := fmt.Sprintf(`{"household_id":%q,"waste_id":%q,"amount":"100"}`,
		householdID, wasteID)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/payments",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestNewValidator_DBExistsPickup_ReturnsFalseWhenNotFound verifies that
// db_exists_pickup rejects a UUID absent from the database.
func TestNewValidator_DBExistsPickup_ReturnsFalseWhenNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	sqlxDB := sqlx.NewDb(db, "postgres")

	_, e := newTestHandlerWithDB(t, sqlxDB)

	householdID := uuid.New()
	wasteID := uuid.New()

	// households lookup returns 1 → passes; waste_pickups lookup returns 0 → fails
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM households`).
		WithArgs(householdID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM waste_pickups`).
		WithArgs(wasteID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	body := fmt.Sprintf(`{"household_id":%q,"waste_id":%q,"amount":"100"}`,
		householdID, wasteID)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/payments",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestReadyCheck_ReturnsReadyWhenBothDepsOK verifies the happy path returns 200.
func TestReadyCheck_ReturnsReadyWhenBothDepsOK(t *testing.T) {
	dbMock, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer func() { _ = dbMock.Close() }()
	mock.ExpectPing()

	sqlxDB := sqlx.NewDb(dbMock, "postgres")

	storageSvc := mocks.NewStorageService(t)
	storageSvc.On("Ping", t.Context()).Return(nil)

	hSvc := mocks.NewHouseholdService(t)
	pSvc := mocks.NewPickupService(t)
	paySvc := mocks.NewPaymentService(t)
	rptSvc := mocks.NewReportService(t)
	h := handler.New(hSvc, pSvc, paySvc, rptSvc, config.Load(), sqlxDB, storageSvc)
	e := echo.New()
	h.RegisterRoutes(e)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ready"`)
	assert.Contains(t, rec.Body.String(), `"db":"ok"`)
	assert.Contains(t, rec.Body.String(), `"storage":"ok"`)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestReadyCheck_StorageUnreachable verifies 503 when storage ping fails.
func TestReadyCheck_StorageUnreachable(t *testing.T) {
	dbMock, mockDB, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer func() { _ = dbMock.Close() }()
	mockDB.ExpectPing()

	sqlxDB := sqlx.NewDb(dbMock, "postgres")

	storageSvc := mocks.NewStorageService(t)
	storageSvc.On("Ping", t.Context()).Return(fmt.Errorf("storage down"))

	hSvc := mocks.NewHouseholdService(t)
	pSvc := mocks.NewPickupService(t)
	paySvc := mocks.NewPaymentService(t)
	rptSvc := mocks.NewReportService(t)
	h := handler.New(hSvc, pSvc, paySvc, rptSvc, config.Load(), sqlxDB, storageSvc)
	e := echo.New()
	h.RegisterRoutes(e)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"unready"`)
	assert.Contains(t, rec.Body.String(), `"storage":"unreachable"`)
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
