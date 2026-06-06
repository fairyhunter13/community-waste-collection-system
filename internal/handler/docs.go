package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/fairyhunter13/community-waste-collection-system/internal/apispec"
)

// ServeOpenAPISpec serves the embedded OpenAPI 3.0 specification as YAML.
func (h *Handler) ServeOpenAPISpec(c echo.Context) error {
	return c.Blob(http.StatusOK, "application/yaml; charset=utf-8", apispec.Spec)
}

// ServeSwaggerUI redirects the browser to the Swagger UI CDN pre-loaded with our spec.
func (h *Handler) ServeSwaggerUI(c echo.Context) error {
	specURL := c.Scheme() + "://" + c.Request().Host + "/api/docs/openapi.yaml"
	html := `<!DOCTYPE html>
<html>
<head><title>Community Waste Collection API — Docs</title></head>
<body>
<script>
window.location = "https://petstore.swagger.io/?url=` + specURL + `";
</script>
<p>Redirecting to Swagger UI… or <a href="https://petstore.swagger.io/?url=` + specURL + `">click here</a>.</p>
</body>
</html>`
	return c.HTML(http.StatusOK, html)
}
