// Package dashboards contains Grafana dashboard correctness tests.
// D1: static lint — validates dashboard JSON structure, datasource UIDs,
// template variable references, and PromQL/LogQL expression syntax.
// Requires no running infrastructure.
package dashboards_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	promparser "github.com/prometheus/prometheus/promql/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	extract "github.com/fairyhunter13/community-waste-collection-system/test/dashboards/internal"
)

// TestDashboardLint validates every dashboard JSON in deployments/grafana/dashboards/.
func TestDashboardLint(t *testing.T) {
	paths, err := extract.GlobDashboards()
	require.NoError(t, err, "glob dashboards")
	require.NotEmpty(t, paths, "no dashboard files found")

	declaredUIDs := loadDatasourceUIDs(t)
	t.Logf("datasource UIDs declared: %v", setKeys(declaredUIDs))

	for _, path := range paths {
		path := path
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d, err := extract.Load(path)
			require.NoError(t, err, "parse dashboard JSON")
			require.NotEmpty(t, d.UID, "dashboard must have a uid")
			require.NotEmpty(t, d.Title, "dashboard must have a title")

			panels := extract.AllPanels(d)
			declaredVars := extract.DeclaredVars(d)

			t.Logf("dashboard %q: %d panels, %d template vars",
				d.Title, len(panels), len(d.Tmpl.List))

			var promqlCount, logqlCount int
			for _, p := range panels {
				// Datasource UID check — only when the panel has an explicit datasource
				if p.Datasource.UID != "" && !strings.HasPrefix(p.Datasource.UID, "${") {
					assert.Contains(t, declaredUIDs, p.Datasource.UID,
						"panel %q references undeclared datasource UID %q", p.Title, p.Datasource.UID)
				}

				for _, target := range p.Targets {
					// Determine effective datasource type: target-level overrides panel-level.
					dsType := p.Datasource.Type
					if target.Datasource.Type != "" {
						dsType = target.Datasource.Type
					}
					isLoki := dsType == "loki"

					if expr := strings.TrimSpace(target.Expr); expr != "" {
						if isLoki {
							logqlCount++
							validateLogQL(t, p.Title, expr, declaredVars)
						} else {
							promqlCount++
							validatePromQL(t, p.Title, expr, declaredVars)
						}
					}
					// Legacy LogQL field used by older Loki panels
					if query := strings.TrimSpace(target.Query); query != "" {
						logqlCount++
						validateLogQL(t, p.Title, query, declaredVars)
					}
				}
			}
			t.Logf("expressions validated: %d PromQL, %d LogQL", promqlCount, logqlCount)
		})
	}
}

// validatePromQL parses expr using the Prometheus PromQL parser and checks
// that any $variable references in expr are declared in declaredVars.
func validatePromQL(t *testing.T, panelTitle, expr string, declaredVars map[string]struct{}) {
	t.Helper()

	// Substitute Grafana variables with a valid literal before parsing so the
	// PromQL parser does not reject them.
	clean := grafanaVarRegex.ReplaceAllString(expr, `"__var__"`)
	// Replace label-value interpolation patterns like ${var:regex} too
	clean = grafanaVarBraceRegex.ReplaceAllString(clean, `"__var__"`)

	_, err := promparser.NewParser(promparser.Options{}).ParseExpr(clean)
	assert.NoError(t, err,
		"panel %q: PromQL parse failed for expr %q (cleaned: %q)", panelTitle, expr, clean)

	// Check declared variables
	for _, match := range grafanaVarRegex.FindAllString(expr, -1) {
		varName := strings.TrimPrefix(strings.TrimSuffix(match, "}"), "${")
		varName = strings.TrimPrefix(varName, "$")
		// Strip modifiers like :regex, :glob etc.
		if idx := strings.IndexByte(varName, ':'); idx != -1 {
			varName = varName[:idx]
		}
		if _, ok := declaredVars[varName]; !ok {
			// Some variables are Grafana built-ins (__interval, __rate_interval, etc.)
			if !strings.HasPrefix(varName, "__") {
				assert.Failf(t, "undeclared variable",
					"panel %q expr references undeclared variable $%s", panelTitle, varName)
			}
		}
	}
}

// validateLogQL performs lightweight structural validation of a LogQL expression:
// it must contain at least one stream selector `{...}` and all `{...}` blocks
// must be syntactically closed.
func validateLogQL(t *testing.T, panelTitle, expr string, declaredVars map[string]struct{}) {
	t.Helper()

	assert.True(t, strings.Contains(expr, "{"),
		"panel %q: LogQL expression has no stream selector: %q", panelTitle, expr)

	// Count balanced braces
	openCount, closeCount := strings.Count(expr, "{"), strings.Count(expr, "}")
	assert.Equal(t, openCount, closeCount,
		"panel %q: unbalanced braces in LogQL expression %q", panelTitle, expr)
}

// loadDatasourceUIDs parses all YAML files in the datasources provisioning
// directory and collects every datasource UID.
func loadDatasourceUIDs(t *testing.T) map[string]struct{} {
	t.Helper()

	type dsEntry struct {
		UID  string `yaml:"uid"`
		Name string `yaml:"name"`
		Type string `yaml:"type"`
	}
	type dsFile struct {
		Datasources []dsEntry `yaml:"datasources"`
	}

	dir := extract.DatasourcesDir()
	uids := make(map[string]struct{})

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}
		var f dsFile
		if unmarshalErr := yaml.Unmarshal(b, &f); unmarshalErr != nil {
			return fmt.Errorf("parse %s: %w", path, unmarshalErr)
		}
		for _, ds := range f.Datasources {
			if ds.UID != "" {
				uids[ds.UID] = struct{}{}
			}
			// Grafana also accepts the datasource name as a UID in some contexts
			if ds.Name != "" {
				uids[ds.Name] = struct{}{}
			}
		}
		return nil
	})
	require.NoError(t, err, "walk datasources dir")
	return uids
}

// TestDashboardsParseAsJSON is a quick smoke test that all dashboard files
// are valid JSON (separate from the structural checks).
func TestDashboardsParseAsJSON(t *testing.T) {
	paths, err := extract.GlobDashboards()
	require.NoError(t, err)
	require.NotEmpty(t, paths)

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			b, err := os.ReadFile(path)
			require.NoError(t, err)
			var m map[string]any
			assert.NoError(t, json.Unmarshal(b, &m), "must parse as JSON")
		})
	}
}

var (
	// Matches $variable or ${variable} or ${variable:modifier}.
	grafanaVarRegex      = regexp.MustCompile(`\$\{?[a-zA-Z_][a-zA-Z0-9_:]*\}?`)
	grafanaVarBraceRegex = regexp.MustCompile(`\$\{[^}]+\}`)
)

func setKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
