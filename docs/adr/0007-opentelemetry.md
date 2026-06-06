# ADR 0007 — OpenTelemetry for vendor-neutral distributed tracing

**Status:** Accepted

## Context

We want to ship traces without binding the application to a specific
trace backend. Jaeger is fine for local development, but production
operators may prefer Tempo, Honeycomb, Datadog, or another OTLP-aware
collector.

## Decision

Instrument with OpenTelemetry Go SDK and export over OTLP to a local
`otel-collector`. The collector forwards to Jaeger in dev. `otelecho`
middleware creates the root HTTP span automatically; each service,
repository, worker, and storage function creates named child spans
with domain attributes (e.g. `pickup.id`, `payment.status`).
Handlers use `trace.SpanFromContext` for enrichment so we never create
duplicate spans for a single request.

## Consequences

- Trace backend can be swapped by changing the OTLP endpoint in the
  collector config — no code changes.
- Span enrichment is idempotent: handlers add attributes to the
  existing root span instead of creating new ones.
- We pay an extra hop (app → collector → backend) in dev, accepted in
  exchange for the deployment flexibility.
