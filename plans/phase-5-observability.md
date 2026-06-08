# Phase 5 — Observability

Three correlated signal types: structured logs, distributed traces, and Prometheus metrics.
All three share a `trace_id` so any log line can be linked to the originating trace.

## Structured logging

`internal/observability/logger.go` builds a `*slog.Logger` writing JSON to stdout.
Every handler injects `request_id`, `trace_id`, and `span_id` into the logger stored
in the request context. Service and repository functions retrieve it via
`observability.FromContext(ctx)`.

Log levels:
- `INFO` — successful operations with key domain attributes (pickup_id, payment_id, etc.)
- `WARN` — business-rule rejections (BR-01 pending payment, BR-03 safety check, etc.)
- `ERROR` — unexpected repository/storage failures.
- `DEBUG` — entry/exit breadcrumbs in high-frequency paths.

## Distributed tracing

`internal/observability/tracer.go` initialises an OTel trace provider that exports via
OTLP/HTTP to Jaeger's native receiver (`OTEL_EXPORTER_OTLP_ENDPOINT`, default
`http://jaeger:4318`). No intermediary collector.

TLS is controlled by `OTEL_EXPORTER_OTLP_TLS_INSECURE` (default `true` — insecure
for local dev; set to `false` in production).

Every handler, service, repository, worker, and storage function creates a named child
span, e.g.:
- `service.pickup.Create`, `service.pickup.Complete`
- `repository.household.FindByID`, `repository.pickup.UpdateStatus`
- `storage.s3.Upload`

Span attributes include domain identifiers (`pickup.id`, `household.id`, `payment.amount`)
and DB metadata (`db.system`, `db.operation`, `db.sql.table`).

## Prometheus metrics

`internal/observability/metrics.go` registers 14 instruments at package init:

| Instrument | Type | Labels |
|-----------|------|--------|
| `http_requests_total` | Counter | method, path, status |
| `http_request_duration_seconds` | Histogram | method, path |
| `db_query_duration_seconds` | Histogram | table, operation |
| `db_errors_total` | Counter | table, operation |
| `pickups_created_total` | Counter | type |
| `pickups_completed_total` | Counter | type |
| `pickups_canceled_total` | Counter | reason |
| `payments_created_total` | Counter | — |
| `payments_confirmed_total` | Counter | — |
| `worker_ticks_total` | Counter | — |
| `worker_canceled_total` | Counter | — |
| `worker_errors_total` | Counter | — |
| `s3_upload_duration_seconds` | Histogram | — |
| `s3_upload_errors_total` | Counter | — |

Scraped by Prometheus at `:2112/metrics`. Two Grafana dashboards
(`deployments/grafana/dashboards/`) are version-controlled and provisioned
automatically on `docker compose up`.

## Readiness probe

`GET /readyz` (`internal/handler/health.go:ReadyCheck`) checks both the DB (ping)
and MinIO (HeadBucket via `StorageService.Ping`). Returns HTTP 200 with
`{"db":"ok","storage":"ok"}` or HTTP 503 with an `"unreachable"` marker for any
failed dependency. Load balancers should gate traffic on `/readyz`.

## Verification

- `go test ./internal/observability/...` — logger and tracer unit tests.
- Unit test U2 (`internal/handler/health_test.go`) — `/readyz` DB-down returns 503.
- Unit test U3 — `/readyz` storage-down returns 503.
- `make test-e2e` — E2E tests assert `trace_id` appears in log output.
- `make dashboards-lint` — Grafana dashboard JSON schema and metric-name correctness.
