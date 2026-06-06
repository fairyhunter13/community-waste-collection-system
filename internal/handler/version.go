package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// VersionInfo is the payload returned by GET /api/version. Values are
// populated at build time via ldflags `-X main.version=… -X main.commit=…
// -X main.buildDate=…` and forwarded by main.SetVersionInfo. Empty fields
// mean the binary was built without ldflags (e.g. `go run`).
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

var buildInfo = VersionInfo{
	Version:   "dev",
	Commit:    "unknown",
	BuildDate: "unknown",
}

// SetVersionInfo overrides the build metadata returned by /api/version.
// Intended to be called once from main() with the linker-injected values.
func SetVersionInfo(version, commit, buildDate string) {
	if version != "" {
		buildInfo.Version = version
	}
	if commit != "" {
		buildInfo.Commit = commit
	}
	if buildDate != "" {
		buildInfo.BuildDate = buildDate
	}
}

// Version returns the embedded build metadata as JSON.
func (h *Handler) Version(c echo.Context) error {
	return c.JSON(http.StatusOK, buildInfo)
}
