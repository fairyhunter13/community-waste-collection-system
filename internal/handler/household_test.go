package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/handler"
	"github.com/fairyhunter13/community-waste-collection-system/internal/mocks"
)

type HouseholdHandlerSuite struct {
	suite.Suite
	echo   *echo.Echo
	h      *handler.Handler
	hSvc   *mocks.HouseholdService
	pSvc   *mocks.PickupService
	paySvc *mocks.PaymentService
	rptSvc *mocks.ReportService
}

func (s *HouseholdHandlerSuite) SetupTest() {
	s.echo = echo.New()
	s.hSvc = mocks.NewHouseholdService(s.T())
	s.pSvc = mocks.NewPickupService(s.T())
	s.paySvc = mocks.NewPaymentService(s.T())
	s.rptSvc = mocks.NewReportService(s.T())
	s.h = handler.New(s.hSvc, s.pSvc, s.paySvc, s.rptSvc, config.Load())
	s.h.RegisterRoutes(s.echo)
}

func TestHouseholdHandler(t *testing.T) {
	suite.Run(t, new(HouseholdHandlerSuite))
}

func (s *HouseholdHandlerSuite) TestCreateHousehold_201() {
	household := &domain.Household{ID: uuid.New(), OwnerName: "John", Address: "Jl. 1"}
	s.hSvc.On("Create", mock.Anything, mock.AnythingOfType("domain.CreateHouseholdRequest")).
		Return(household, nil)

	body := `{"owner_name":"John","address":"Jl. 1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/households", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	var resp map[string]any
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &resp))
	s.True(resp["success"].(bool))
}

func (s *HouseholdHandlerSuite) TestCreateHousehold_400_InvalidJSON() {
	req := httptest.NewRequest(http.MethodPost, "/api/households", strings.NewReader(`not json`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *HouseholdHandlerSuite) TestGetHousehold_200() {
	id := uuid.New()
	household := &domain.Household{ID: id, OwnerName: "Jane"}
	s.hSvc.On("GetByID", mock.Anything, id).Return(household, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/households/"+id.String(), nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)
}

func (s *HouseholdHandlerSuite) TestGetHousehold_404() {
	id := uuid.New()
	s.hSvc.On("GetByID", mock.Anything, id).Return(nil, domain.ErrNotFound)

	req := httptest.NewRequest(http.MethodGet, "/api/households/"+id.String(), nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *HouseholdHandlerSuite) TestListHouseholds_200() {
	households := []*domain.Household{{ID: uuid.New()}}
	s.hSvc.On("List", mock.Anything, 1, 20).Return(households, 1, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/households", nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	var resp map[string]any
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &resp))
	s.NotNil(resp["meta"])
}

func (s *HouseholdHandlerSuite) TestDeleteHousehold_204() {
	id := uuid.New()
	s.hSvc.On("Delete", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/households/"+id.String(), nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *HouseholdHandlerSuite) TestDeleteHousehold_404() {
	id := uuid.New()
	s.hSvc.On("Delete", mock.Anything, id).Return(domain.ErrNotFound)

	req := httptest.NewRequest(http.MethodDelete, "/api/households/"+id.String(), nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusNotFound, rec.Code)
}
