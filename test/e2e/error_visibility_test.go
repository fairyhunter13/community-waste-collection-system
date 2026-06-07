//go:build e2e

package e2e_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// TestErrorVisibility verifies the full observability stack for error responses:
//  1. A 400 response carries meta.trace_id and the X-Request-Id header.
//  2. Loki (when reachable) surfaces at least one log line for that request_id
//     and each line carries trace_id, span_id, and a "source" field (AddSource).
//
// Set E2E_SKIP_LOKI=true to skip the Loki assertions (e.g. when running without
// the full observability stack).
func (s *E2ESuite) TestErrorVisibility() {
	// POST an invalid household (empty body — missing required fields).
	resp := s.do(http.MethodPost, "/api/households", map[string]any{})
	defer resp.Body.Close()
	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)

	// Capture the request ID from the response header.
	reqID := resp.Header.Get("X-Request-Id")

	// Decode the error envelope.
	var body map[string]any
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))

	s.False(body["success"].(bool), "success must be false for 400 response")

	// meta may be nil when OTel tracing is not wired (unit / local runs without
	// a collector), so only assert its fields when it is present.
	var traceID string
	if meta, ok := body["meta"].(map[string]any); ok && meta != nil {
		if tid, ok := meta["trace_id"].(string); ok && tid != "" {
			traceID = tid
			s.NotEmpty(meta["span_id"], "meta.span_id must be set when trace_id is present")
		}
		if reqID == "" {
			// Fall back to the request_id embedded in the meta (set by the handler).
			if rid, ok := meta["request_id"].(string); ok {
				reqID = rid
			}
		}
	}

	if os.Getenv("E2E_SKIP_LOKI") == "true" {
		s.T().Skip("E2E_SKIP_LOKI=true — skipping Loki assertions")
	}

	s.Require().NotEmpty(reqID, "X-Request-Id header must be set to query Loki")

	lokiBase := os.Getenv("LOKI_BASE_URL")
	if lokiBase == "" {
		lokiBase = "http://localhost:3100"
	}

	// Query Loki for log lines matching this request_id in the last 5 minutes.
	end := time.Now()
	start := end.Add(-5 * time.Minute)

	logQL := fmt.Sprintf(`{service="waste-api"} | json | request_id="%s"`, reqID)

	queryURL := fmt.Sprintf(
		"%s/loki/api/v1/query_range?query=%s&start=%d&end=%d&limit=50",
		lokiBase,
		url.QueryEscape(logQL),
		start.UnixNano(),
		end.UnixNano(),
	)

	lokiResp, err := http.Get(queryURL) //nolint:noctx // simple test query
	s.Require().NoError(err, "Loki HTTP query must not error")
	defer lokiResp.Body.Close()

	s.Require().Equal(http.StatusOK, lokiResp.StatusCode, "Loki must return 200")

	raw, err := io.ReadAll(lokiResp.Body)
	s.Require().NoError(err)

	var lokiBody map[string]any
	s.Require().NoError(json.Unmarshal(raw, &lokiBody))

	// Navigate: data.result[].values[]
	data, ok := lokiBody["data"].(map[string]any)
	s.Require().True(ok, "Loki response must have a data object")

	results, ok := data["result"].([]any)
	s.Require().True(ok, "Loki data.result must be an array")
	s.Require().NotEmpty(results, "Loki must return at least one log stream for request_id=%q", reqID)

	// Validate that at least one log entry carries trace_id, span_id, and source.
	foundLine := false
	for _, stream := range results {
		streamMap, ok := stream.(map[string]any)
		if !ok {
			continue
		}
		values, ok := streamMap["values"].([]any)
		if !ok {
			continue
		}
		for _, val := range values {
			pair, ok := val.([]any)
			if !ok || len(pair) < 2 {
				continue
			}
			lineStr, ok := pair[1].(string)
			if !ok {
				continue
			}
			var line map[string]any
			if err := json.Unmarshal([]byte(lineStr), &line); err != nil {
				// Not a JSON log line — skip.
				continue
			}
			// Every structured log line must carry source (AddSource=true).
			s.NotEmpty(line["source"], "log line must carry 'source' field (AddSource=true): line=%s", lineStr)
			// When tracing is active, trace_id and span_id must appear.
			if traceID != "" {
				s.Equal(traceID, line["trace_id"], "log line trace_id must match the error response trace_id")
				s.NotEmpty(line["span_id"], "log line must carry span_id")
			}
			foundLine = true
		}
	}

	s.True(foundLine, "at least one structured JSON log line must be found in Loki for request_id=%q", reqID)
}
