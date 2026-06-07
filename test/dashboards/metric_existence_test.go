//go:build dashboard_int

// D2: metric existence check — verifies every metric name used in a dashboard
// PromQL expression is registered in the app's Prometheus registry.
// Imports internal/observability and internal/middleware to trigger promauto
// registration; no running infrastructure required.
package dashboards_test

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	promparser "github.com/prometheus/prometheus/promql/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Side-effect imports: trigger promauto metric registration.
	_ "github.com/fairyhunter13/community-waste-collection-system/internal/middleware"
	_ "github.com/fairyhunter13/community-waste-collection-system/internal/observability"
	extract "github.com/fairyhunter13/community-waste-collection-system/test/dashboards/internal"
)

// descFQNameRe extracts the fully-qualified metric name from prometheus.Desc.String().
var descFQNameRe = regexp.MustCompile(`fqName: "([^"]+)"`)

// TestMetricExistence checks that every metric name referenced in a PromQL
// expression across all dashboards is registered in the app's Prometheus
// registry. Uses Describe() so unobserved Vec metrics (CounterVec etc.) are
// included even before any label values have been recorded.
func TestMetricExistence(t *testing.T) {
	registered := buildRegisteredSet(t)
	t.Logf("registered metric base names: %d", len(registered)/4) // approx

	paths, err := extract.GlobDashboards()
	require.NoError(t, err)
	require.NotEmpty(t, paths)

	var missing []string
	for _, path := range paths {
		d, err := extract.Load(path)
		require.NoError(t, err)

		for _, p := range extract.AllPanels(d) {
			dsType := p.Datasource.Type
			for _, target := range p.Targets {
				if target.Datasource.Type != "" {
					dsType = target.Datasource.Type
				}
				if dsType == "loki" {
					continue
				}
				expr := strings.TrimSpace(target.Expr)
				if expr == "" {
					continue
				}
				for _, name := range extractMetricNames(expr) {
					if _, ok := registered[name]; !ok {
						missing = append(missing, fmt.Sprintf(
							"dashboard %q panel %q: metric %q not registered",
							filepath.Base(path), p.Title, name,
						))
					}
				}
			}
		}
	}

	assert.Empty(t, missing,
		"metrics referenced in dashboards but not registered:\n%s",
		strings.Join(missing, "\n"))
}

// buildRegisteredSet uses Describe() to enumerate all metric descriptors in the
// default registry (including unobserved Vec metrics). For each base name it
// also inserts _bucket, _count, _sum variants so histogram references like
// foo_bucket match the registered base name foo.
func buildRegisteredSet(t *testing.T) map[string]struct{} {
	t.Helper()

	type describer interface {
		Describe(chan<- *prometheus.Desc)
	}
	rd, ok := prometheus.DefaultGatherer.(describer)
	require.True(t, ok, "prometheus.DefaultGatherer must implement Describe")

	ch := make(chan *prometheus.Desc, 10000)
	go func() {
		rd.Describe(ch)
		close(ch)
	}()

	set := make(map[string]struct{})
	for d := range ch {
		m := descFQNameRe.FindStringSubmatch(d.String())
		if len(m) < 2 || m[1] == "" {
			continue
		}
		base := m[1]
		set[base] = struct{}{}
		// Expand so histogram/summary derived names are covered.
		set[base+"_bucket"] = struct{}{}
		set[base+"_count"] = struct{}{}
		set[base+"_sum"] = struct{}{}
	}
	return set
}

// extractMetricNames parses a PromQL expression and returns the distinct
// VectorSelector metric names it references. Grafana template variables are
// substituted with a placeholder before parsing.
func extractMetricNames(expr string) []string {
	clean := grafanaVarRegex.ReplaceAllString(expr, `"__var__"`)
	clean = grafanaVarBraceRegex.ReplaceAllString(clean, `"__var__"`)

	node, err := promparser.NewParser(promparser.Options{}).ParseExpr(clean)
	if err != nil {
		return nil
	}

	var names []string
	seen := make(map[string]struct{})
	promparser.Inspect(node, func(n promparser.Node, _ []promparser.Node) error {
		vs, ok := n.(*promparser.VectorSelector)
		if ok && vs.Name != "" {
			if _, dup := seen[vs.Name]; !dup {
				seen[vs.Name] = struct{}{}
				names = append(names, vs.Name)
			}
		}
		return nil
	})
	return names
}
