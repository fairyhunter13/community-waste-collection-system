package middleware_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/middleware"
)

func TestRateLimit_PassesThrough(t *testing.T) {
	cfg := &config.Config{RateLimitRPS: 100, RateLimitBurst: 100}
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.1.1:1234"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	called := false
	h := middleware.RateLimiter(cfg)(func(c echo.Context) error {
		called = true
		return c.NoContent(http.StatusOK)
	})

	require.NoError(t, h(c))
	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRateLimit_Returns429WhenExhausted(t *testing.T) {
	cfg := &config.Config{RateLimitRPS: 0, RateLimitBurst: 0}
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.2.1:1234"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	called := false
	h := middleware.RateLimiter(cfg)(func(c echo.Context) error {
		called = true
		return c.NoContent(http.StatusOK)
	})

	require.NoError(t, h(c))
	assert.False(t, called)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestRecoverMiddleware_PanicReturns500(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := middleware.RecoverMiddleware(logger)(func(c echo.Context) error {
		panic("boom")
	})

	require.NoError(t, h(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRecoverMiddleware_NoPanicPassesThrough(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := middleware.RecoverMiddleware(logger)(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	require.NoError(t, h(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequestLogger_LogsAndPassesThrough(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test-path", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := middleware.RequestLogger(logger)(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	require.NoError(t, h(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	out := buf.String()
	assert.Contains(t, out, "GET")
	assert.Contains(t, out, "/test-path")
}

func TestOtelTrace_PassesThrough(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := middleware.OtelTrace("test-service")(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	require.NoError(t, h(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequestMetrics_RecordsSuccessfulRequest(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/test")

	called := false
	h := middleware.RequestMetrics()(func(c echo.Context) error {
		called = true
		return c.NoContent(http.StatusOK)
	})

	require.NoError(t, h(c))
	assert.True(t, called)
}

func TestRequestMetrics_EmptyPathUsesUnknown(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// c.Path() returns "" when no route is matched

	h := middleware.RequestMetrics()(func(c echo.Context) error {
		return c.NoContent(http.StatusCreated)
	})

	require.NoError(t, h(c))
}
