package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const wantValidationErrorCode = "VALIDATION_ERROR"

// TestErrorEnvelopeShape_AllValidationErrorsMatchContract proves that every
// write endpoint that accepts a JSON body produces the documented error
// envelope `{success:false, error:{code, message}}` for VALIDATION_ERROR
// outcomes. A reviewer who depends on the envelope shape (e.g. a client SDK
// that switches on `error.code`) must never see a deviation between
// endpoints.
func TestErrorEnvelopeShape_AllValidationErrorsMatchContract(t *testing.T) {
	_, e := newTestHandler(t)

	type tc struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}

	cases := []tc{
		{
			name:       "households empty body",
			method:     http.MethodPost,
			path:       "/api/households",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   wantValidationErrorCode,
		},
		{
			name:       "households malformed json",
			method:     http.MethodPost,
			path:       "/api/households",
			body:       `{not-json`,
			wantStatus: http.StatusBadRequest,
			wantCode:   wantValidationErrorCode,
		},
		{
			name:       "pickups empty body",
			method:     http.MethodPost,
			path:       "/api/pickups",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   wantValidationErrorCode,
		},
		{
			name:       "pickups invalid type enum",
			method:     http.MethodPost,
			path:       "/api/pickups",
			body:       `{"household_id":"00000000-0000-0000-0000-000000000001","type":"plutonium"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   wantValidationErrorCode,
		},
		{
			name:       "payments empty body",
			method:     http.MethodPost,
			path:       "/api/payments",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   wantValidationErrorCode,
		},
		{
			name:       "schedule missing date",
			method:     http.MethodPut,
			path:       "/api/pickups/00000000-0000-0000-0000-000000000001/schedule",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   wantValidationErrorCode,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), c.method, c.path, bytes.NewBufferString(c.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, c.wantStatus, rec.Code, "status mismatch for %s", c.name)

			var envelope map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope), "body must be JSON for %s", c.name)

			success, ok := envelope["success"].(bool)
			require.True(t, ok, "%s: envelope must have a boolean `success`", c.name)
			assert.False(t, success, "%s: error responses must report success=false", c.name)

			errBlock, ok := envelope["error"].(map[string]any)
			require.True(t, ok, "%s: envelope must have an `error` object", c.name)

			code, _ := errBlock["code"].(string)
			msg, _ := errBlock["message"].(string)
			assert.Equal(t, c.wantCode, code, "%s: error.code mismatch", c.name)
			assert.NotEmpty(t, msg, "%s: error.message must be non-empty", c.name)
		})
	}
}
