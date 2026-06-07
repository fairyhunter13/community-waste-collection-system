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

// ServeSwaggerUI serves a self-contained Swagger UI page that loads the
// OpenAPI spec directly from this host — no external redirects required.
func (h *Handler) ServeSwaggerUI(c echo.Context) error {
	specURL := c.Scheme() + "://" + c.Request().Host + "/api/docs/openapi.yaml"
	html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Community Waste Collection API — Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
<script>
window.onload = function() {
  SwaggerUIBundle({
    url: "` + specURL + `",
    dom_id: '#swagger-ui',
    presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
    layout: "BaseLayout",
    deepLinking: true
  });
};
</script>
</body>
</html>`
	return c.HTML(http.StatusOK, html)
}
