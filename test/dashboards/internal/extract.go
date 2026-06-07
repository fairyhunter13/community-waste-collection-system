// Package extract provides helpers for reading Grafana dashboard JSON and
// extracting PromQL/LogQL expressions for validation tests.
package extract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// Dashboard is a minimal representation of a Grafana dashboard JSON file,
// sufficient for extracting panel targets and template variables.
type Dashboard struct {
	UID    string       `json:"uid"`
	Title  string       `json:"title"`
	Panels []Panel      `json:"panels"`
	Tmpl   Templating   `json:"templating"`
}

// Panel represents a single Grafana panel (row panels recurse into sub-panels).
type Panel struct {
	ID          int      `json:"id"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Datasource  DSRef    `json:"datasource"`
	Targets     []Target `json:"targets"`
	Panels      []Panel  `json:"panels"` // row-type panels contain sub-panels
}

// DSRef holds a datasource reference (may be a UID string or an object).
type DSRef struct {
	UID  string `json:"uid"`
	Type string `json:"type"`
}

func (d *DSRef) UnmarshalJSON(b []byte) error {
	// Datasource can be a plain string UID or {"uid":"...","type":"..."}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		d.UID = s
		return nil
	}
	type plain DSRef
	return json.Unmarshal(b, (*plain)(d))
}

// Target is one query target within a panel.
type Target struct {
	Datasource DSRef  `json:"datasource"` // per-target datasource override (optional)
	Expr       string `json:"expr"`       // PromQL or LogQL expression
	Query      string `json:"query"`      // LogQL expression (legacy Loki field)
}

// Templating holds the dashboard template variable list.
type Templating struct {
	List []TemplateVar `json:"list"`
}

// TemplateVar is one $variable definition in the dashboard.
type TemplateVar struct {
	Name string `json:"name"`
}

// Load reads and parses a Grafana dashboard JSON file.
func Load(path string) (*Dashboard, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d Dashboard
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// AllPanels returns every panel in the dashboard, recursing into row panels.
func AllPanels(d *Dashboard) []Panel {
	var out []Panel
	for _, p := range d.Panels {
		out = append(out, p)
		out = append(out, p.Panels...) // one level of nesting (rows)
	}
	return out
}

// DeclaredVars returns the set of template variable names declared in a dashboard.
func DeclaredVars(d *Dashboard) map[string]struct{} {
	m := make(map[string]struct{})
	for _, v := range d.Tmpl.List {
		m[v.Name] = struct{}{}
	}
	return m
}

// DashboardsDir returns the absolute path to deployments/grafana/dashboards/
// relative to the module root (located by searching up from the calling file).
func DashboardsDir() string {
	_, file, _, _ := runtime.Caller(0)
	// file is .../test/dashboards/internal/extract.go — module root is three dirs up
	root := filepath.Join(filepath.Dir(file), "..", "..", "..")
	return filepath.Join(root, "deployments", "grafana", "dashboards")
}

// DatasourcesDir returns the absolute path to deployments/grafana/provisioning/datasources/.
func DatasourcesDir() string {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..")
	return filepath.Join(root, "deployments", "grafana", "provisioning", "datasources")
}

// GlobDashboards returns paths to all *.json files in the dashboards directory.
func GlobDashboards() ([]string, error) {
	return filepath.Glob(filepath.Join(DashboardsDir(), "*.json"))
}
