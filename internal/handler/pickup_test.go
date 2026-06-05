package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/handler"
	"github.com/fairyhunter13/community-waste-collection-system/internal/mocks"
)

type PickupHandlerSuite struct {
	suite.Suite
	echo   *echo.Echo
	h      *handler.Handler
	hSvc   *mocks.HouseholdService
	pSvc   *mocks.PickupService
	paySvc *mocks.PaymentService
	rptSvc *mocks.ReportService
}

func (s *PickupHandlerSuite) SetupTest() {
	s.echo = echo.New()
	s.hSvc = mocks.NewHouseholdService(s.T())
	s.pSvc = mocks.NewPickupService(s.T())
	s.paySvc = mocks.NewPaymentService(s.T())
	s.rptSvc = mocks.NewReportService(s.T())
	s.h = handler.New(s.hSvc, s.pSvc, s.paySvc, s.rptSvc, config.Load())
	s.h.RegisterRoutes(s.echo)
}

func TestPickupHandler(t *testing.T) {
	suite.Run(t, new(PickupHandlerSuite))
}

func (s *PickupHandlerSuite) TestCreatePickup_201() {
	householdID := uuid.New()
	pickup := &domain.WastePickup{ID: uuid.New(), HouseholdID: householdID, Type: domain.WasteTypeOrganic, Status: domain.PickupStatusPending}
	s.pSvc.On("Create", mock.Anything, mock.AnythingOfType("domain.CreatePickupRequest")).Return(pickup, nil)

	body := fmt.Sprintf(`{"household_id":"%s","type":"organic"}`, householdID)
	req := httptest.NewRequest(http.MethodPost, "/api/pickups", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusCreated, rec.Code)
}

func (s *PickupHandlerSuite) TestCreatePickup_409_PendingPayment() {
	s.pSvc.On("Create", mock.Anything, mock.Anything).Return(nil, domain.ErrConflict)

	householdID := uuid.New()
	body := fmt.Sprintf(`{"household_id":"%s","type":"organic"}`, householdID)
	req := httptest.NewRequest(http.MethodPost, "/api/pickups", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusConflict, rec.Code)
}

func (s *PickupHandlerSuite) TestListPickups_200() {
	pickups := []*domain.WastePickup{{ID: uuid.New()}}
	s.pSvc.On("List", mock.Anything, mock.AnythingOfType("domain.PickupFilter")).Return(pickups, 1, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/pickups", nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)
}

func (s *PickupHandlerSuite) TestSchedulePickup_200() {
	id := uuid.New()
	pickup := &domain.WastePickup{ID: id, Status: domain.PickupStatusScheduled}
	s.pSvc.On("Schedule", mock.Anything, id, mock.AnythingOfType("domain.SchedulePickupRequest")).Return(pickup, nil)

	body := `{"pickup_date":"2026-12-01T10:00:00Z"}`
	req := httptest.NewRequest(http.MethodPut, "/api/pickups/"+id.String()+"/schedule", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)
}

func (s *PickupHandlerSuite) TestSchedulePickup_422_ElectronicNoSafetyCheck() {
	id := uuid.New()
	s.pSvc.On("Schedule", mock.Anything, id, mock.Anything).Return(nil, domain.ErrBusinessRule)

	body := fmt.Sprintf(`{"pickup_date":"%s"}`, time.Now().Add(24*time.Hour).Format(time.RFC3339))
	req := httptest.NewRequest(http.MethodPut, "/api/pickups/"+id.String()+"/schedule", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusUnprocessableEntity, rec.Code)
}

func (s *PickupHandlerSuite) TestCompletePickup_200() {
	id := uuid.New()
	pickup := &domain.WastePickup{ID: id, Status: domain.PickupStatusCompleted}
	s.pSvc.On("Complete", mock.Anything, id).Return(pickup, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/pickups/"+id.String()+"/complete", nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)
}

func (s *PickupHandlerSuite) TestCancelPickup_200() {
	id := uuid.New()
	pickup := &domain.WastePickup{ID: id, Status: domain.PickupStatusCanceled}
	s.pSvc.On("Cancel", mock.Anything, id).Return(pickup, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/pickups/"+id.String()+"/cancel", nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	var resp map[string]any
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &resp))
	s.True(resp["success"].(bool))
}

func (s *PickupHandlerSuite) TestCancelPickup_409_Completed() {
	id := uuid.New()
	s.pSvc.On("Cancel", mock.Anything, id).Return(nil, domain.ErrConflict)

	req := httptest.NewRequest(http.MethodPut, "/api/pickups/"+id.String()+"/cancel", nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusConflict, rec.Code)
}
