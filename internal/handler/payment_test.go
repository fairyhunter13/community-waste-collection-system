package handler_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
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

type PaymentHandlerSuite struct {
	suite.Suite
	echo   *echo.Echo
	h      *handler.Handler
	hSvc   *mocks.HouseholdService
	pSvc   *mocks.PickupService
	paySvc *mocks.PaymentService
	rptSvc *mocks.ReportService
}

func (s *PaymentHandlerSuite) SetupTest() {
	s.echo = echo.New()
	s.hSvc = mocks.NewHouseholdService(s.T())
	s.pSvc = mocks.NewPickupService(s.T())
	s.paySvc = mocks.NewPaymentService(s.T())
	s.rptSvc = mocks.NewReportService(s.T())
	s.h = handler.New(s.hSvc, s.pSvc, s.paySvc, s.rptSvc, config.Load(), nil)
	s.h.RegisterRoutes(s.echo)
}

func TestPaymentHandler(t *testing.T) {
	suite.Run(t, new(PaymentHandlerSuite))
}

func (s *PaymentHandlerSuite) TestCreatePayment_201() {
	householdID := uuid.New()
	wasteID := uuid.New()
	payment := &domain.Payment{ID: uuid.New(), HouseholdID: householdID, WasteID: wasteID, Amount: decimal.RequireFromString("50000.00")}
	s.paySvc.On("Create", mock.Anything, mock.AnythingOfType("domain.CreatePaymentRequest")).Return(payment, nil)

	body := `{"household_id":"` + householdID.String() + `","waste_id":"` + wasteID.String() + `","amount":"50000.00"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/payments", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusCreated, rec.Code)
}

func (s *PaymentHandlerSuite) TestListPayments_200() {
	payments := []*domain.Payment{{ID: uuid.New()}}
	s.paySvc.On("List", mock.Anything, mock.AnythingOfType("domain.PaymentFilter")).Return(payments, 1, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/payments", nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)
}

func (s *PaymentHandlerSuite) TestConfirmPayment_200() {
	id := uuid.New()
	payment := &domain.Payment{ID: id, Status: domain.PaymentStatusPaid}
	s.paySvc.On("Confirm", mock.Anything, id, mock.Anything, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).
		Return(payment, nil)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("proof", "receipt.jpg")
	s.Require().NoError(err)
	_, err = part.Write([]byte("fake-image-data"))
	s.Require().NoError(err)
	s.Require().NoError(mw.Close())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/payments/"+id.String()+"/confirm", &buf)
	req.Header.Set(echo.HeaderContentType, mw.FormDataContentType())
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)
}

func (s *PaymentHandlerSuite) TestConfirmPayment_400_NoFile() {
	id := uuid.New()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/payments/"+id.String()+"/confirm", nil)
	req.Header.Set(echo.HeaderContentType, "multipart/form-data; boundary=xxx")
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *PaymentHandlerSuite) TestCreatePayment_400_InvalidJSON() {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/payments", strings.NewReader(`not json`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *PaymentHandlerSuite) TestCreatePayment_400_NegativeAmount() {
	body := `{"household_id":"` + uuid.New().String() + `","waste_id":"` + uuid.New().String() + `","amount":"-100"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/payments", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *PaymentHandlerSuite) TestCreatePayment_400_NonNumericAmount() {
	body := `{"household_id":"` + uuid.New().String() + `","waste_id":"` + uuid.New().String() + `","amount":"abc"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/payments", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *PaymentHandlerSuite) TestListPayments_400_InvalidStatus() {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/payments?status=garbage", nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *PaymentHandlerSuite) TestListPayments_400_InvalidDateFrom() {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/payments?date_from=not-a-date", nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *PaymentHandlerSuite) TestListPayments_400_InvalidDateTo() {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/payments?date_to=not-a-date", nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *PaymentHandlerSuite) TestListPayments_200_WithDateRange() {
	payments := []*domain.Payment{{ID: uuid.New()}}
	s.paySvc.On("List", mock.Anything, mock.AnythingOfType("domain.PaymentFilter")).Return(payments, 1, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/payments?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)
}

func (s *PaymentHandlerSuite) TestConfirmPayment_404() {
	id := uuid.New()
	s.paySvc.On("Confirm", mock.Anything, id, mock.Anything, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).
		Return(nil, domain.ErrNotFound)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("proof", "receipt.jpg")
	s.Require().NoError(err)
	_, err = part.Write([]byte("fake-image-data"))
	s.Require().NoError(err)
	s.Require().NoError(mw.Close())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/payments/"+id.String()+"/confirm", &buf)
	req.Header.Set(echo.HeaderContentType, mw.FormDataContentType())
	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	s.Equal(http.StatusNotFound, rec.Code)
}
