package observability

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "waste-collection-api"

// Tracer returns the application-wide OTel tracer. Call after InitTracer has set the provider.
func Tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

// SpanFail records err on span and marks the span failed.
//
// RecordError alone leaves the span's status Unset, so a trace shows the exception event
// without the span reading as an error — the two calls have to travel together.
func SpanFail(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
