package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/community-waste-collection-system/internal/middleware"
)

func TestRequestID_GeneratesWhenAbsent(t *testing.T) {
	e := echo.New()
	e.Use(middleware.RequestID())
	e.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	rid := rec.Header().Get(echo.HeaderXRequestID)
	assert.NotEmpty(t, rid, "X-Request-ID must be set on response")
}

func TestRequestID_HonoursInboundHeader(t *testing.T) {
	const supplied = "00000000-0000-0000-0000-000000000abc"

	e := echo.New()
	e.Use(middleware.RequestID())
	e.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.Header.Set(echo.HeaderXRequestID, supplied)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, supplied, rec.Header().Get(echo.HeaderXRequestID))
}

func TestRequestID_ExposesValueOnRequestForDownstream(t *testing.T) {
	var seenOnRequest string

	e := echo.New()
	e.Use(middleware.RequestID())
	e.GET("/x", func(c echo.Context) error {
		seenOnRequest = c.Request().Header.Get(echo.HeaderXRequestID)
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, seenOnRequest, "downstream handlers must see X-Request-ID on the request")
	assert.Equal(t, seenOnRequest, rec.Header().Get(echo.HeaderXRequestID))
}
