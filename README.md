[![CI](https://github.com/fairyhunter13/community-waste-collection-system/actions/workflows/ci.yml/badge.svg)](https://github.com/fairyhunter13/community-waste-collection-system/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/fairyhunter13/community-waste-collection-system/graph/badge.svg)](https://codecov.io/gh/fairyhunter13/community-waste-collection-system)

# Community Waste Collection API

A RESTful API service for managing community household waste collection,
pickup scheduling, and payment processing.

Built with Go 1.26, Echo v4, PostgreSQL 17, MinIO, and Docker.

---

## Key Features

- **16 REST endpoints** across households, pickups, payments, and reports
- **6 business rules** enforced in the service layer:
  - BR-01 — A household with any pending payment cannot create a new pickup
  - BR-02 — Only pending pickups can be scheduled; only scheduled can be completed or cancelled
  - BR-03 — Electronic waste pickup requires a `safety_check: true` flag
  - BR-04 — Organic pickups with no scheduled date for 3 days are auto-cancelled by a background worker
  - BR-05 — Completing a pickup atomically auto-generates a payment record at the confirmed amount
  - BR-06 — Payment confirmation requires a multipart proof-of-payment file upload
- **Per-IP rate limiting** on pickup creation (5 req/s, burst 10) via token bucket
- **Full-stack observability**: structured JSON logs (slog), distributed tracing (OTel → Jaeger), 14 Prometheus metrics, 2 auto-provisioned Grafana dashboards
- **Test coverage >80%** enforced in CI; integration tests use real PostgreSQL via testcontainers
- **OpenAPI 3.0 spec** embedded in the binary and served at `/api/docs/openapi.yaml`

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [API Walkthrough](#api-walkthrough)
- [Error Reference](#error-reference)
- [Environment Variables](#environment-variables)
- [Running Locally (without Docker)](#running-locally-without-docker)
- [Running Tests](#running-tests)
- [Data Model](#data-model)
- [API Reference](#api-reference)
- [Observability](#observability)
- [Architecture](#architecture)
- [Architecture Decisions](#architecture-decisions)

---

## Prerequisites

- Go 1.26+
- Docker + Docker Compose
- `make`
- (Optional) [`migrate` CLI](https://github.com/golang-migrate/migrate) for manual migration runs

---

## Quick Start

```bash
cp .env.example .env
make docker-up       # start all services (postgres, minio, otel-collector, jaeger, prometheus, grafana, api)
make migrate-up      # apply database migrations
```

Verify the stack is healthy:

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

| Service | URL | Credentials |
|---|---|---|
| API | http://localhost:8080 | — |
| OpenAPI Spec | http://localhost:8080/api/docs/openapi.yaml | — |
| Swagger UI | http://localhost:8080/api/docs | — |
| Grafana Dashboard | http://localhost:3000 | admin / admin |
| Prometheus | http://localhost:9090 | — |
| Jaeger UI (traces) | http://localhost:16686 | — |
| MinIO console | http://localhost:9001 | minioadmin / minioadmin |
| Prometheus metrics | http://localhost:2112/metrics | — |
| pprof debug | http://localhost:6060/debug/pprof/ | — |

Grafana auto-provisions two dashboards on startup:

- **Waste Collection API** — 7 rows: API traffic, business events, database performance, background worker, Go runtime, process metrics, S3 storage, and Jaeger traces
- **Business Operations** — 4 rows: pickup funnel, payment funnel, error breakdown, S3 storage KPIs

---

## API Walkthrough

A complete end-to-end flow from household registration to payment confirmation:

**1. Register a household**

```bash
HH=$(curl -s -X POST http://localhost:8080/api/households \
  -H 'Content-Type: application/json' \
  -d '{"owner_name":"Ahmad Sutrisno","address":"Jl. Merdeka 12, Jakarta"}')
echo $HH | jq .
HH_ID=$(echo $HH | jq -r '.data.id')
```

```json
{ "success": true, "data": { "id": "uuid-here", "owner_name": "Ahmad Sutrisno", "address": "Jl. Merdeka 12, Jakarta", "created_at": "..." } }
```

**2. Request a pickup**

```bash
PK=$(curl -s -X POST http://localhost:8080/api/pickups \
  -H 'Content-Type: application/json' \
  -d "{\"household_id\":\"$HH_ID\",\"type\":\"organic\"}")
PK_ID=$(echo $PK | jq -r '.data.id')
```

**3. Schedule the pickup**

```bash
curl -s -X PUT "http://localhost:8080/api/pickups/$PK_ID/schedule" \
  -H 'Content-Type: application/json' \
  -d '{"pickup_date":"2026-06-15T09:00:00Z"}' | jq .
```

**4. Complete the pickup** (auto-creates a payment record)

```bash
curl -s -X PUT "http://localhost:8080/api/pickups/$PK_ID/complete" | jq .
```

**5. Get the auto-created payment**

```bash
PM=$(curl -s "http://localhost:8080/api/payments?household_id=$HH_ID")
PM_ID=$(echo $PM | jq -r '.data[0].id')
```

**6. Confirm payment with proof upload**

```bash
curl -s -X PUT "http://localhost:8080/api/payments/$PM_ID/confirm" \
  -F "proof=@/path/to/receipt.jpg;type=image/jpeg" | jq .
```

**7. View household history**

```bash
curl -s "http://localhost:8080/api/reports/households/$HH_ID/history" | jq .
```

---

## Error Reference

| HTTP | Code | Triggered by |
|---|---|---|
| `400` | `VALIDATION_ERROR` | Missing required field, invalid enum value, past pickup date, malformed UUID |
| `404` | `NOT_FOUND` | Resource ID does not exist in the database |
| `409` | `CONFLICT` | BR-01 (household has a pending payment), BR-02 (wrong pickup status for operation) |
| `422` | `BUSINESS_RULE_VIOLATION` | BR-03 (electronic without safety_check), BR-06 (confirm without proof file) |
| `429` | `RATE_LIMITED` | More than 5 pickup creation requests per second from the same IP |
| `500` | `INTERNAL_ERROR` | Unexpected server-side error |
| `503` | `service unavailable` | Health check: database unreachable |

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `APP_PORT` | `8080` | HTTP server port |
| `APP_ENV` | `development` | Environment name |
| `DEBUG_PORT` | `6060` | pprof debug server port |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/waste_collection?sslmode=disable` | PostgreSQL connection string |
| `DB_MAX_OPEN_CONNS` | `25` | DB connection pool max open |
| `DB_MAX_IDLE_CONNS` | `10` | DB connection pool max idle |
| `DB_CONN_MAX_IDLE_TIME` | `5m` | Max time a connection can remain idle before being closed |
| `S3_ENDPOINT` | `http://localhost:9000` | S3-compatible storage endpoint |
| `S3_BUCKET` | `waste-proofs` | Bucket for payment proof uploads |
| `S3_ACCESS_KEY` | `minioadmin` | S3 access key |
| `S3_SECRET_KEY` | `minioadmin` | S3 secret key |
| `S3_REGION` | `us-east-1` | S3 region (required by AWS SDK; MinIO ignores it) |
| `S3_USE_PATH_STYLE` | `true` | Use path-style S3 URLs (required for MinIO) |
| `MAX_UPLOAD_SIZE_MB` | `10` | Maximum proof file upload size |
| `RATE_LIMIT_RPS` | `5` | Pickup creation rate limit (req/sec/IP) |
| `RATE_LIMIT_BURST` | `10` | Rate limit burst capacity |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | Log format: `json` or `text` |
| `METRICS_PORT` | `2112` | Prometheus metrics server port |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | OTel OTLP HTTP endpoint |
| `OTEL_SERVICE_NAME` | `community-waste-collection-api` | OTel service name |
| `OTEL_SERVICE_VERSION` | `0.1.0` | OTel service version tag on all spans |
| `WORKER_CANCEL_INTERVAL` | `1h` | How often the organic-canceler worker runs |
| `WORKER_ORGANIC_CUTOFF_DAYS` | `3` | Days after which a pending organic pickup is auto-cancelled |
| `CODECOV_TOKEN` | — | Codecov upload token (CI only, never commit the value) |

See `.env.example` for a complete reference with comments.

---

## Running Locally (without Docker)

```bash
# 1. Start infrastructure only
docker compose up -d postgres minio otel-collector

# 2. Apply migrations
make migrate-up

# 3. (Optional) Seed demo data
make seed   # or: psql "$DATABASE_URL" -f scripts/seed.sql

# 4. Run the API
make run
```

---

## Running Tests

```bash
# Unit tests (no external dependencies)
make test

# Integration tests (spins up Postgres via testcontainers)
make test-integration

# E2E tests (requires full stack via docker-compose)
make docker-up && make migrate-up
make test-e2e

# HTTP performance benchmarks (requires full stack + running app)
make docker-up && make migrate-up
make perf

# DB-layer micro-benchmarks (requires DATABASE_URL, no docker stack needed)
make bench
```

> The BR-04 organic worker E2E test requires `E2E_DB_URL` pointing at the host-accessible
> Postgres URL (e.g. `postgres://postgres:postgres@localhost:5432/waste_collection?sslmode=disable`).
> Without it the worker test skips automatically.

---

## Data Model

```
households
  id          UUID PK
  owner_name  TEXT NOT NULL
  address     TEXT NOT NULL
  created_at  TIMESTAMPTZ
  updated_at  TIMESTAMPTZ
      │
      │ 1:N (CASCADE DELETE)
      ▼
waste_pickups
  id             UUID PK
  household_id   UUID FK → households.id
  type           ENUM (organic, non-organic, hazardous, electronic)
  status         ENUM (pending, scheduled, completed, canceled)
  pickup_date    TIMESTAMPTZ NULL
  safety_check   BOOL (electronic only)
  created_at     TIMESTAMPTZ
  updated_at     TIMESTAMPTZ
      │
      │ 1:1 (CASCADE DELETE)
      ▼
payments
  id             UUID PK
  waste_pickup_id UUID FK → waste_pickups.id
  household_id   UUID FK → households.id
  amount         NUMERIC(12,2)
  status         ENUM (pending, paid, failed)
  proof_url      TEXT NULL
  created_at     TIMESTAMPTZ
  updated_at     TIMESTAMPTZ
```

Deleting a household cascades to all its pickups, which cascade to all their payments.

---

## API Reference

### Health

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Returns `{"status":"ok"}` or 503 if DB is unreachable |

### Documentation

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/docs/openapi.yaml` | OpenAPI 3.0.3 specification (YAML) |
| `GET` | `/api/docs` | Redirect to Swagger UI |

### Households

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/households` | Register a new household |
| `GET` | `/api/households` | List all households (paginated) |
| `GET` | `/api/households/:id` | Get a household by ID |
| `DELETE` | `/api/households/:id` | Delete a household |

### Waste Pickups

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/pickups` | Request a new waste pickup |
| `GET` | `/api/pickups` | List pickups (filterable by status, household) |
| `PUT` | `/api/pickups/:id/schedule` | Schedule a pickup date |
| `PUT` | `/api/pickups/:id/complete` | Mark pickup as completed |
| `PUT` | `/api/pickups/:id/cancel` | Cancel a pickup |

**Rate limiting:** Pickup creation is rate-limited to 5 req/s per IP (burst 10). Excess requests receive HTTP 429.

### Payments

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/payments` | Create a payment record |
| `GET` | `/api/payments` | List payments (filterable by household, status, date range) |
| `PUT` | `/api/payments/:id/confirm` | Confirm payment with proof file upload (multipart/form-data, field: `proof`) |

### Reports

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/reports/waste-summary` | Waste pickup counts grouped by type and status |
| `GET` | `/api/reports/payment-summary` | Revenue totals grouped by payment status |
| `GET` | `/api/reports/households/:id/history` | Full pickup and payment history for a household |

### Common Response Format

Success:
```json
{ "success": true, "data": { ... } }
```

List (with pagination):
```json
{ "success": true, "data": [...], "meta": { "page": 1, "per_page": 20, "total": 5, "total_pages": 1 } }
```

Error:
```json
{ "success": false, "error": { "code": "NOT_FOUND", "message": "not found" } }
```

---

## Observability

### Service URLs

| Service | URL | Purpose |
|---|---|---|
| API | http://localhost:8080 | Application endpoints |
| Health | http://localhost:8080/health | Liveness probe |
| Prometheus metrics | http://localhost:2112/metrics | Raw metric scrape target |
| Prometheus UI | http://localhost:9090 | Query and alert UI |
| Grafana | http://localhost:3000 | Dashboards (admin/admin) |
| Jaeger | http://localhost:16686 | Distributed trace search |
| pprof | http://localhost:6060/debug/pprof/ | CPU/memory profiling |

### Prometheus Metrics (14 instruments)

| Metric | Type | Description |
|---|---|---|
| `http_requests_total` | Counter | HTTP requests by method, path, status |
| `http_request_duration_seconds` | Histogram | HTTP request latency |
| `waste_pickups_created_total` | Counter | Pickups created by type |
| `waste_pickups_completed_total` | Counter | Pickups completed by type |
| `waste_pickups_canceled_total` | Counter | Pickups canceled by type and reason (manual/auto) |
| `waste_organic_auto_cancels_total` | Counter | Organic pickups auto-cancelled by background worker |
| `waste_payments_created_total` | Counter | Payment records created |
| `waste_payments_confirmed_total` | Counter | Payments confirmed with proof |
| `db_query_duration_seconds` | Histogram | DB query latency by table and operation |
| `db_errors_total` | Counter | DB errors by table and operation |
| `worker_cycles_total` | Counter | Background worker execution cycles |
| `worker_cycle_duration_seconds` | Histogram | Worker cycle duration |
| `worker_expired_found_total` | Counter | Expired organic pickups found per cycle |
| `s3_upload_duration_seconds` | Histogram | S3 upload latency |
| `s3_upload_bytes_total` | Counter | Total bytes uploaded to S3 |
| `s3_errors_total` | Counter | S3 operation errors |

### Distributed Tracing

Every request receives a root span created by `otelecho`. All handler, service, repository, worker, and storage functions create child spans. Business attributes (pickup type, household ID, filter params, etc.) are set on the root span via `trace.SpanFromContext`. Traces are exported via OTLP HTTP to the OpenTelemetry Collector and forwarded to Jaeger.

Span naming convention: `layer.domain.Method` (e.g., `service.pickup.Create`, `repository.household.FindByID`, `storage.s3.Upload`).

---

## Architecture

```
cmd/api/
  main.go             ← DI wiring + graceful shutdown

api/
  openapi.yaml        ← OpenAPI 3.0 specification (embedded in binary)

internal/
  apispec/            ← Go embed wrapper for openapi.yaml
  config/             ← env-based configuration
  domain/             ← entities, interfaces, sentinel errors
  handler/            ← Echo HTTP handlers + response helpers
  middleware/         ← rate limiter, logger, recover, OTel trace
  service/            ← business logic + all 6 business rules
  repository/         ← sqlx SQL implementations
  storage/            ← S3/MinIO upload client
  observability/      ← slog logger, OTel tracer, Prometheus metrics
  worker/             ← background organic-pickup auto-canceler
  mocks/              ← testify/mockery mocks (generated)

migrations/           ← golang-migrate SQL files (up + down)
test/e2e/             ← end-to-end tests (build tag: e2e)
test/perf/            ← HTTP performance benchmarks (build tag: perf)
scripts/seed.sql      ← demo seed data
deployments/          ← Docker Compose, Prometheus config, Grafana provisioning
```

**Request flow:** `handler` → `service` → `repository` → PostgreSQL

Dependencies only flow inward. Domain interfaces decouple layers.

---

## Architecture Decisions

### No ORM — raw SQL via sqlx
SQL is explicit, reviewable, and optimised per query. The `sqlx` library adds struct scanning without abstraction overhead.

### Sentinel errors for domain outcomes
Five sentinel errors (`ErrNotFound`, `ErrConflict`, `ErrBusinessRule`, `ErrValidation`, `ErrInternalFailure`) allow the handler layer to map outcomes to HTTP status codes via `errors.Is`, without coupling service logic to HTTP.

### `shopspring/decimal` for monetary amounts
Amounts are stored as PostgreSQL `NUMERIC(12,2)` and scanned directly into `decimal.Decimal` from `github.com/shopspring/decimal`. The type implements `database/sql.Scanner` natively, eliminates floating-point representation issues, and marshals to JSON as a quoted string (`"50000.00"`) matching the wire format.

### Per-IP token bucket rate limiting
`golang.org/x/time/rate` provides a per-IP `rate.Limiter` stored in a `sync.Map`. This enforces the pickup-creation rate limit without an external dependency like Redis.

### Background worker with context cancellation
The `OrganicCanceler` worker uses `time.Ticker` and listens on a context, enabling clean shutdown via `context.WithCancel` coordinated by the main function's signal handler.

### Business rules enforced in the service layer
All 6 business rules (BR-01 through BR-06) live in the service layer, keeping handlers thin and repository implementations focused on data access only.

### OpenTelemetry — vendor-neutral distributed tracing
OTel was chosen over direct Jaeger/Zipkin SDKs so the trace backend can be swapped by changing the OTLP endpoint. `otelecho` middleware creates root HTTP spans automatically; each service, repository, worker, and storage function creates named child spans with domain attributes. Span enrichment in handlers uses `trace.SpanFromContext` (a no-op when no span is active) to add business attributes without creating duplicate spans.

### Prometheus + Grafana — RED metrics with auto-provisioning
`promauto` registers metrics at package init, eliminating the need to pass a registry through dependency injection. Metrics follow the RED pattern (Rate, Errors, Duration) for HTTP and database layers. Both Grafana dashboards and datasources are version-controlled in `deployments/grafana/` and auto-provisioned on container startup — no manual dashboard import required.
