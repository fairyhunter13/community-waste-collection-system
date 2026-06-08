package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/handler"
	"github.com/fairyhunter13/community-waste-collection-system/internal/mocks"
)

type ReportHandlerSuite struct {
	suite.Suite
	echo   *echo.Echo
	h      *handler.Handler
	hSvc   *mocks.HouseholdService
	pSvc   *mocks.PickupService
	paySvc *mocks.PaymentService
	rptSvc *mocks.ReportService
}

func (s *ReportHandlerSuite) SetupTest() {
	s.echo = echo.New()
	s.hSvc = mocks.NewHouseholdService(s.T())
	s.pSvc = mocks.NewPickupService(s.T())
	s.paySvc = mocks.NewPaymentService(s.T())
	s.rptSvc = mocks.NewReportService(s.T())
	s.h = handler.New(s.hSvc, s.pSvc, s.paySvc, s.rptSvc, config.Load(), nil, nil)
	s.h.RegisterRoutes(s.echo)
}

func TestReportHandler(t *testing.T) {
	suite.Run(t, new(ReportHandlerSuite))
}

func (s *ReportHandlerSuite) TestWasteSummary_200() {
	summaries := []domain.WasteTypeSummary{{Type: domain.WasteTypeOrganic, Total: 3}}
	s.rptSvc.On("WasteSummary", mock.Anything).Return(summaries, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/reports/waste-summary", nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)
}

func (s *ReportHandlerSuite) TestPaymentSummary_200() {
	result := &domain.PaymentSummaryResult{TotalRevenue: decimal.RequireFromString("150000")}
	s.rptSvc.On("PaymentSummary", mock.Anything).Return(result, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/reports/payment-summary", nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)
}

func (s *ReportHandlerSuite) TestHouseholdHistory_200() {
	id := uuid.New()
	history := &domain.HouseholdHistoryResult{Household: &domain.Household{ID: id}}
	s.rptSvc.On("HouseholdHistory", mock.Anything, id).Return(history, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/reports/households/"+id.String()+"/history", nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)
}

func (s *ReportHandlerSuite) TestHouseholdHistory_404() {
	id := uuid.New()
	s.rptSvc.On("HouseholdHistory", mock.Anything, id).Return(nil, domain.ErrNotFound)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/reports/households/"+id.String()+"/history", nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusNotFound, rec.Code)
}
