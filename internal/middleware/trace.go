package middleware

import (
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

// OtelTrace returns an Echo middleware that propagates OpenTelemetry trace context.
func OtelTrace(serviceName string) echo.MiddlewareFunc {
	return otelecho.Middleware(serviceName)
}
