package observability

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "waste-collection-api"

// Tracer returns the application-wide OTel tracer. Call after InitTracer has set the provider.
func Tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}
