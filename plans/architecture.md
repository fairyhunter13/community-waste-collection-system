# Architecture

## Layer responsibilities

```
cmd/api/main.go         DI wiring, graceful shutdown, signal handling
internal/config/        Env-var configuration (validated on startup)
internal/domain/        Entities, service/repository interfaces, sentinel errors
internal/handler/       Echo HTTP handlers, request parsing, response envelope
internal/middleware/    Rate limiter, request ID injection, OTel trace, logger
internal/service/       All business logic — BR-01..BR-06 enforcement
internal/repository/    sqlx SQL implementations — no business logic
internal/storage/       S3/MinIO upload client
internal/observability/ slog logger, OTel tracer, Prometheus metrics registration
internal/worker/        Background organic-pickup auto-canceler (BR-04)
internal/mocks/         Testify/mockery generated mocks for domain interfaces
migrations/             golang-migrate SQL files (numbered up + down pairs)
test/e2e/               End-to-end tests (build tag: e2e)
test/perf/              HTTP performance benchmarks (build tag: perf)
test/load/              k6 load-test scenarios
test/dashboards/        Grafana dashboard correctness suite (lint, metrics, E2E, Playwright)
deployments/            Docker Compose, Prometheus config, Grafana provisioning
```

Dependencies only flow inward. Domain interfaces decouple layers so each
can be unit-tested in isolation using the generated mocks.

## Business rules

| # | Rule | Enforcement location |
|---|------|----------------------|
| BR-01 | Household with a pending payment cannot create a new pickup | `service/pickup.go` Create — `pg_advisory_xact_lock` + partial UNIQUE index `uq_payments_one_pending_per_household` |
| BR-02 | Only pending pickups can be scheduled; only scheduled can be completed or cancelled | Conditional `UPDATE … WHERE status=?` → `ErrConflict` on wrong status |
| BR-03 | Electronic pickup requires `safety_check: true` before scheduling | Service validation → `ErrBusinessRule` → 422 |
| BR-04 | Organic pickups not scheduled within 3 days are auto-cancelled | `worker/organic_canceler.go` — ticks on `WORKER_CANCEL_INTERVAL`, exits cleanly on context cancel |
| BR-05 | Completing a pickup atomically auto-creates a payment record | `SELECT … FOR UPDATE` + `BEGIN/COMMIT` transaction in `service/pickup.go` Complete |
| BR-06 | Payment confirmation requires a multipart proof-of-payment file upload | MIME allowlist + magic-byte sniff + MinIO upload in `service/payment.go` Confirm |

## Request flow

```
Client → Echo middleware stack
       → handler (parse + validate input)
       → service (enforce BRs, compose transactions)
       → repository (execute SQL via sqlx)
       → PostgreSQL
       ↳ S3/MinIO (payment proof upload only)
```

## Observability

Three signal types, all correlated via `trace_id`:

- **Metrics** — 14 Prometheus instruments registered at package init in
  `internal/observability/metrics.go`. Scraped at `:2112/metrics`. Two
  Grafana dashboards auto-provisioned under `deployments/grafana/`.
- **Logs** — `log/slog` JSON to stdout. Every line carries `trace_id`,
  `span_id`, `request_id`, `op`. Promtail tails container stdout → Loki.
- **Traces** — OTel Go SDK, OTLP/HTTP export to Jaeger's native receiver
  (`jaeger:4318`). No intermediary collector. All handler, service,
  repository, worker, and storage functions create named child spans.

## Database

PostgreSQL 17 with five migrations:

| # | Migration | Purpose |
|---|-----------|---------|
| 000001 | Create tables | Baseline schema — households, waste_pickups, payments |
| 000002 | Add indexes | Lookup indexes on foreign keys |
| 000003 | Enum changes | Waste type enum updates |
| 000004 | Unique pending payment | Partial UNIQUE index `uq_payments_one_pending_per_household` (BR-01 DB-level guard) |
| 000005 | Performance indexes | Composite indexes for list + filter queries |

## Configuration

All configuration via environment variables. See `internal/config/config.go`
for defaults and validation. Key tunables:

- `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST` — pickup creation rate limit
- `WORKER_CANCEL_INTERVAL` / `WORKER_ORGANIC_CUTOFF_DAYS` — BR-04 timing
- `MAX_UPLOAD_SIZE_MB` — proof file cap
- `OTEL_EXPORTER_OTLP_ENDPOINT` — Jaeger OTLP address

## Graceful shutdown

`cmd/api/main.go` catches SIGINT/SIGTERM, cancels the root context
(worker drain), then calls `e.Shutdown(ctx)` (HTTP drain within
`HTTPShutdownTimeout`). Both the HTTP server and the background worker
participate in a `sync.WaitGroup` so the process does not exit until
in-flight work is complete.
