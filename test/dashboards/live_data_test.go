//go:build dashboard_e2e

// D3: live-data check — after smoke traffic has been generated, queries every
// PromQL panel expression against a running Prometheus instance and asserts at
// least one non-zero, non-NaN result per panel.
//
// Requires env vars:
//   PROMETHEUS_URL — base URL of a running Prometheus (default http://localhost:9090)
//   LOKI_URL       — base URL of a running Loki (default http://localhost:3100)
//
// The test is designed to run after the E2E smoke-traffic sequence has fired
// enough requests to populate the RED metrics.
package dashboards_test

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	extract "github.com/fairyhunter13/community-waste-collection-system/test/dashboards/internal"
)

// panels listed here are expected to be empty (no traffic on these paths yet).
var allowEmptyPanels = map[string]struct{}{
	"Total Auto-Cancelled Organic Pickups": {},
	"Organic Pickup Auto-Cancel Rate":      {},
	"DB Errors Rate by Table/Operation":    {},
	"Cumulative DB Errors":                 {},
	"S3 Upload Error Rate":                 {},
}

func TestLiveDashboardData(t *testing.T) {
	promURL := envOr("PROMETHEUS_URL", "http://localhost:9090")
	lokiURL := envOr("LOKI_URL", "http://localhost:3100")

	paths, err := extract.GlobDashboards()
	require.NoError(t, err)
	require.NotEmpty(t, paths)

	client := &http.Client{Timeout: 15 * time.Second}

	var totalPanels, nonEmpty, exempted, silent int

	for _, path := range paths {
		d, err := extract.Load(path)
		require.NoError(t, err)

		for _, p := range extract.AllPanels(d) {
			for _, target := range p.Targets {
				dsType := p.Datasource.Type
				if target.Datasource.Type != "" {
					dsType = target.Datasource.Type
				}

				if dsType == "loki" {
					expr := strings.TrimSpace(target.Expr)
					if expr == "" {
						expr = strings.TrimSpace(target.Query)
					}
					if expr == "" {
						continue
					}
					totalPanels++
					if _, exempt := allowEmptyPanels[p.Title]; exempt {
						exempted++
						continue
					}
					if lokiHasData(t, client, lokiURL, expr) {
						nonEmpty++
					} else {
						silent++
						t.Errorf("dashboard %q panel %q: Loki query returned no data: %q",
							filepath.Base(path), p.Title, expr)
					}
				} else {
					expr := strings.TrimSpace(target.Expr)
					if expr == "" {
						continue
					}
					totalPanels++
					if _, exempt := allowEmptyPanels[p.Title]; exempt {
						exempted++
						continue
					}
					if promHasData(t, client, promURL, expr) {
						nonEmpty++
					} else {
						silent++
						t.Errorf("dashboard %q panel %q: PromQL query returned no non-zero data: %q",
							filepath.Base(path), p.Title, expr)
					}
				}
			}
		}
		t.Logf("dashboard %q: panels checked=%d non-empty=%d exempted=%d silent=%d",
			filepath.Base(path), totalPanels, nonEmpty, exempted, silent)
	}
}

// promHasData queries Prometheus instant-query API and returns true if any
// result has a non-zero, non-NaN value.
func promHasData(t *testing.T, c *http.Client, baseURL, expr string) bool {
	t.Helper()

	// Substitute Grafana variables with something that won't break the query
	clean := substituteVars(expr)

	u := baseURL + "/api/v1/query?" + url.Values{"query": {clean}}.Encode()
	resp, err := c.Get(u)
	if err != nil {
		t.Logf("prometheus query error for %q: %v", expr, err)
		return false
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Value []any `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.Status != "success" {
		return false
	}

	for _, r := range result.Data.Result {
		if len(r.Value) < 2 {
			continue
		}
		s, ok := r.Value[1].(string)
		if !ok {
			continue
		}
		if s == "NaN" || s == "+Inf" || s == "-Inf" {
			continue
		}
		var v float64
		if _, err := fmt.Sscanf(s, "%f", &v); err == nil && !math.IsNaN(v) && v != 0 {
			return true
		}
	}
	return false
}

// lokiHasData queries Loki query_range API and returns true if any log line was
// returned. Uses a 5-minute window ending now.
func lokiHasData(t *testing.T, c *http.Client, baseURL, expr string) bool {
	t.Helper()

	clean := substituteVars(expr)
	now := time.Now()
	start := now.Add(-5 * time.Minute)

	u := baseURL + "/loki/api/v1/query_range?" + url.Values{
		"query": {clean},
		"start": {fmt.Sprintf("%d", start.Unix())},
		"end":   {fmt.Sprintf("%d", now.Unix())},
		"limit": {"1"},
	}.Encode()

	resp, err := c.Get(u)
	if err != nil {
		t.Logf("loki query error for %q: %v", expr, err)
		return false
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Values [][]string `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.Status != "success" {
		return false
	}
	for _, r := range result.Data.Result {
		if len(r.Values) > 0 {
			return true
		}
	}
	return false
}

// substituteVars replaces Grafana $variable / ${variable} patterns with
// literal values suitable for instant evaluation.
func substituteVars(expr string) string {
	s := grafanaVarRegex.ReplaceAllStringFunc(expr, func(m string) string {
		name := strings.TrimPrefix(strings.TrimSuffix(m, "}"), "${")
		name = strings.TrimPrefix(name, "$")
		if idx := strings.IndexByte(name, ':'); idx != -1 {
			name = name[:idx]
		}
		switch name {
		case "interval", "__interval", "__rate_interval":
			return "5m"
		default:
			return ".*"
		}
	})
	s = grafanaVarBraceRegex.ReplaceAllString(s, ".*")
	return s
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
