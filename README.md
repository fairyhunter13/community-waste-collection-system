[![CI](https://github.com/fairyhunter13/community-waste-collection-system/actions/workflows/ci.yml/badge.svg)](https://github.com/fairyhunter13/community-waste-collection-system/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/fairyhunter13/community-waste-collection-system/graph/badge.svg)](https://codecov.io/gh/fairyhunter13/community-waste-collection-system)
[![Coverage](https://img.shields.io/codecov/c/github/fairyhunter13/community-waste-collection-system/main)](https://codecov.io/gh/fairyhunter13/community-waste-collection-system)

# Community Waste Collection API

> **Backend engineering test deliverable** — Community Waste Collection API.  
> Implements the supplied brief end-to-end: **16 REST endpoints**, **6 business rules** (BR-01 .. BR-06), and **6 production-readiness requirements** (TR-1 .. TR-6).  
> Traceability matrix: [`plans/phase-7-final-verification.md`](plans/phase-7-final-verification.md)

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
- **Unit test coverage 82.7%** enforced in CI (gate ≥80%); integration tests use real PostgreSQL via testcontainers
- **OpenAPI 3.0 spec** embedded in the binary and served at `/api/docs/openapi.yaml`

---

## Table of Contents — Quick Links

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Troubleshooting](#troubleshooting)
- [API Walkthrough](#api-walkthrough)
- [API Reference](#api-reference)
- [Observability](#observability)
- [Architecture](#architecture)
- [Architecture Decisions](#architecture-decisions) — [ADR index](docs/adr/)

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

- **Go 1.26+** (`go version` should report `go1.26.x`).
- **Docker 24+** with the **Compose v2 plugin** (`docker compose version` should report `2.x`). The legacy `docker-compose` shim is not used.
- **GNU make** (`make --version`). On macOS, ship Xcode Command Line Tools.
- **Ports free** on the host: `8080` (API), `5432` (Postgres), `9000`/`9001` (MinIO), `3000` (Grafana), `9090` (Prometheus), `16686` (Jaeger UI), `4317`/`4318` (OTLP), `2112` (Prometheus scrape target), `6060` (pprof). See [Troubleshooting](#troubleshooting) if any of these collide with a host service.
- **Optional but recommended for local SQL work:**
  - [`migrate` CLI](https://github.com/golang-migrate/migrate/releases) — run migrations from the host without entering the API container.
  - `psql` client — inspect the live DB during development.
  - [`newman`](https://github.com/postmanlabs/newman) (`npm i -g newman`) — replay the Postman collection against a running stack.

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

## Troubleshooting

Common issues encountered when booting the stack for the first time.

### Port already in use

`docker compose up` fails with `bind: address already in use`. The
service that owns the conflicting port is reported in the error.

```bash
# Find the host process holding the port (example: 5432)
sudo ss -tulpn | grep ':5432 '

# Either stop the offending process or remap the port in
# deployments/docker-compose.yml under the relevant service's
# `ports:` section, then `make docker-down && make docker-up`.
```

### Migrations fail with `connection refused`

Postgres takes a few seconds to accept connections after the container
starts. Wait for the `db` service to become healthy before running
`make migrate-up`:

```bash
docker compose -f deployments/docker-compose.yml ps db | grep healthy
```

If healthy still fails, confirm the `DATABASE_URL` in `.env` points at
`localhost:5432` (not `db:5432`, which is the in-network hostname only
visible to other compose services).

### MinIO bucket missing on first run

The application creates the `proofs` bucket on startup if absent. If
`PUT /api/payments/:id/confirm` returns 500 with `BucketNotFound`, the
boot-time check ran before MinIO finished initialising. Restart the
API container:

```bash
docker compose -f deployments/docker-compose.yml restart api
```

You can also create the bucket manually in the MinIO console at
http://localhost:9001 (login `minioadmin` / `minioadmin`).

### OpenTelemetry collector reports `connection refused` for Jaeger

The `otel-collector` service must wait for `jaeger` to be ready before
exporting traces. The Compose file sets `depends_on: jaeger` with a
healthcheck — if you see the error, restart the collector after Jaeger
finishes booting:

```bash
docker compose -f deployments/docker-compose.yml restart otel-collector
```

### `migrate` CLI not found

Install from the [golang-migrate releases](https://github.com/golang-migrate/migrate/releases).
Alternatively, run migrations from inside the API container:

```bash
docker compose -f deployments/docker-compose.yml exec api \
  migrate -path=/migrations -database "$DATABASE_URL" up
```

### Grafana panels are empty

Generate traffic before checking dashboards — without requests, the
panels have no data points to render:

```bash
for _ in $(seq 1 20); do curl -s http://localhost:8080/health >/dev/null; done
```

Then refresh Grafana. If panels still show "No data", confirm the
Prometheus datasource is configured at http://localhost:9090 (it is
auto-provisioned but may report `unreachable` if Prometheus failed to
boot).

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

### Postman / Insomnia Collections

Two equivalent collection exports live under `api/` — 27 requests across 4 folders (Households, Waste Pickups, Payments, Reports), each with saved response examples:

| Tool | File |
|------|------|
| Postman | `api/community-waste.postman_collection.json` |
| Insomnia v4 | `api/community-waste.insomnia_collection.json` |

Set the `base_url` collection variable to `http://localhost:8080`. Replay against a running stack with [Newman](https://github.com/postmanlabs/newman):

```bash
npm i -g newman
newman run api/community-waste.postman_collection.json \
  --env-var base_url=http://localhost:8080
```

### Health

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Liveness probe — returns `{"status":"ok"}` whenever the process is bound and serving. Does NOT touch the DB. |
| `GET` | `/readyz` | Readiness probe — pings the DB. Returns 200 `{"status":"ready"}` when the DB is reachable, 503 `{"status":"unready"}` otherwise. |

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
| Loki | http://localhost:3100 | Log aggregation (Promtail → Loki) |
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

### Structured Logs

Every layer emits structured JSON logs via `log/slog`. Each log line carries `trace_id` and `span_id` from the active OTel span so logs and traces can be correlated in Grafana.

```json
{
  "time": "2025-07-01T10:23:45.123Z",
  "level": "INFO",
  "msg": "scheduled",
  "op": "PickupService.Schedule",
  "pickup_id": "a1b2c3d4-...",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "request_id": "7e3d1f9a-..."
}
```

Log levels by scenario:

| Level | When |
|-------|------|
| `DEBUG` | Entry into a method with input IDs (development detail) |
| `INFO` | State transitions (created, scheduled, completed, cancelled) and expected domain errors (ErrConflict/ErrNotFound/ErrValidation) that map to 4xx |
| `ERROR` | Unexpected failures that return 500 — DB errors, S3 failures, transaction rollbacks |

### Unified Log/Trace Search

The stack ships Loki + Promtail alongside Prometheus and Jaeger. Promtail tails the app container's Docker logs and pushes JSON-parsed entries to Loki.

**Pivot from Grafana:**
1. Open **Grafana → Dashboards → Logs and Traces**
2. Paste a `trace_id` (from a Jaeger search or an `X-Request-ID` response header) into the **trace_id** template variable — the log panel updates live
3. Use the **span_id** variable to narrow to a single span's logs

**Pivot from Jaeger:**
- Each trace span has a **"View logs"** button (configured via `tracesToLogsV2`) that opens a Loki Explore query for that trace_id

**Pivot from Loki:**
- Each log line's `trace_id` field is a clickable derived field link that opens the Jaeger trace directly

### Error Visibility

When any request fails, the JSON response body includes a `meta` object:

```json
{
  "success": false,
  "error": { "code": "VALIDATION_ERROR", "message": "..." },
  "meta": {
    "request_id": "01HXYZ...",
    "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
    "span_id": "00f067aa0ba902b7"
  }
}
```

**To trace an error end-to-end:**
1. Read `meta.request_id` (or `X-Request-Id` response header) and `meta.trace_id` from the error body.
2. Open **Grafana → Logs and Traces** dashboard → paste `trace_id` into the `trace_id` textbox to see all log lines for that request with `source.file`, `source.function`, and `source.line`.
3. Alternatively paste `request_id` into the `request_id` textbox.
4. Click any `trace_id` value in the log panel → opens the full Jaeger trace.
5. Click any Jaeger span → **View logs** → returns to Loki filtered by that span's ID.

Log severity follows this ladder:
- `DEBUG` — method entry with input IDs (no PII)
- `INFO` — successful state transitions
- `WARN` — expected domain errors (conflict, validation, business-rule violation)
- `ERROR` — unexpected repository or infrastructure failures

### SLOs and Alerts

Service-level objectives ([`deployments/prometheus/alerts.yml`](deployments/prometheus/alerts.yml)):

| SLO | Threshold | Alert |
|-----|-----------|-------|
| HTTP success rate | ≥ 99% over 5 min | `HighHTTPErrorRate` |
| p99 request latency (per route, excl. `/metrics`) | ≤ 500 ms over 5 min | `HighP99Latency` |
| Worker cycle freshness (BR-04) | ≥ 1 cycle per 10 min | `BackgroundWorkerStalled` |
| API scrape availability | scrape up for ≥ 2 min | `APIDown` |

Prometheus loads `alerts.yml` via `rule_files:` in `deployments/prometheus.yml`. View live rules at http://localhost:9090/alerts after `make docker-up`.

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

### Canonical Lifecycle (Mermaid)

End-to-end flow exercising every layer plus BR-04 and BR-05:

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant H as Echo Handler
    participant S as Service
    participant R as Repository
    participant DB as PostgreSQL
    participant W as Worker
    participant M as MinIO/S3

    C->>H: POST /api/households
    H->>S: HouseholdService.Create
    S->>R: INSERT household
    R->>DB: SQL
    DB-->>R: row
    R-->>S: domain.Household
    S-->>H: success
    H-->>C: 201 + envelope

    C->>H: POST /api/pickups (BR-01 advisory lock)
    H->>S: PickupService.Create
    S->>R: HasPendingPaymentForHousehold + INSERT pickup
    R->>DB: pg_advisory_xact_lock + check + insert
    DB-->>R: pickup
    R-->>S: domain.WastePickup
    S-->>H: success
    H-->>C: 201

    C->>H: PUT /api/pickups/:id/schedule (BR-02 conditional UPDATE)
    H->>S: PickupService.Schedule
    S->>R: UPDATE … WHERE id=? AND status='pending'
    R->>DB: SQL
    DB-->>R: 1 row affected
    R-->>S: scheduled pickup
    S-->>H: success
    H-->>C: 200

    C->>H: PUT /api/pickups/:id/complete (BR-05 atomicity)
    H->>S: PickupService.Complete (tx)
    S->>R: SELECT … FOR UPDATE + UPDATE pickup + INSERT payment
    R->>DB: BEGIN; lock; update; insert; COMMIT
    DB-->>R: completed + new pending payment
    R-->>S: pickup + payment
    S-->>H: success
    H-->>C: 200

    Note over W,DB: BR-04 every WORKER_CANCEL_INTERVAL
    W->>DB: SELECT organic + pending + created_at < now-N days
    DB-->>W: expired rows
    W->>DB: UPDATE … SET status='canceled' WHERE status='pending'

    C->>H: PUT /api/payments/:id/confirm (multipart proof)
    H->>S: PaymentService.Confirm (MIME allowlist)
    S->>M: PutObject
    M-->>S: object URL
    S->>R: UPDATE payment SET status='paid', proof_file_url=?
    R->>DB: SQL
    DB-->>R: paid
    R-->>S: payment
    S-->>H: success
    H-->>C: 200
```

### Sample JSON Log Line

Every request emits a structured slog line carrying both the OTel trace
identifier and the request id, so reviewers can pivot from a log line to
the trace in Jaeger or vice versa:

```json
{
  "time": "2026-06-07T08:14:21.317Z",
  "level": "INFO",
  "msg": "request",
  "method": "POST",
  "path": "/api/pickups",
  "status": 201,
  "duration_ms": 38,
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "request_id": "0d8b1a2e-9f6f-4c5b-8c1d-0c2b8b3e4f50",
  "remote_ip": "127.0.0.1"
}
```

---

## Failure Modes

How the system degrades when each upstream becomes unavailable, and how
to recover.

### PostgreSQL down

- **Symptoms:** every write endpoint returns `500 INTERNAL_ERROR`;
  `/readyz` returns `503` (the readiness probe pings the DB on every
  request); the BR-04 worker logs `cycle failed: ...` and increments
  `worker_cycles_failed_total`.
- **What still works:** `/health` (liveness) and `/api/version` —
  neither touches the DB. `/metrics` continues to scrape.
- **Recovery:** restore Postgres → `/readyz` flips green within one
  request; the worker self-heals on the next tick (no manual restart
  needed). The HTTP server does not need to be bounced.

### MinIO / S3 down

- **Symptoms:** `PUT /api/payments/:id/confirm` fails with
  `500 INTERNAL_ERROR` (the proof-file upload step errors out); all
  other endpoints are unaffected.
- **What still works:** every read path, household / pickup / payment
  creation, and the BR-04 worker.
- **Recovery:** restore MinIO and re-issue the confirmation request.
  Idempotency on confirm is enforced via the conditional
  `WHERE status='pending'` UPDATE, so retries are safe.

### Worker goroutine dies

- **Symptoms:** `worker_cycles_total` stops climbing; expired organic
  pickups stop being auto-cancelled (BR-04 paused).
- **Recovery:** Tier 6 added `recover()` around the worker loop body
  so a panic now logs, increments `worker_cycles_failed_total`, and
  the next tick continues. If the loop is permanently stuck, restart
  the container; the worker resumes on boot.

### OTel collector / Jaeger down

- **Symptoms:** traces stop appearing in Jaeger; structured logs still
  carry `trace_id` / `span_id` so the correlation field is preserved.
- **What still works:** every HTTP and worker path. The OTel exporter
  is best-effort and never blocks the request hot path.
- **Recovery:** restore the collector; tracing resumes on the next
  request. No application restart needed.

### Prometheus / Grafana down

- **Symptoms:** dashboards go blank; alerts stop firing.
- **What still works:** the application itself is unaffected; metrics
  continue to be exposed on `/metrics`.
- **Recovery:** restart Prometheus; it back-fills from the application
  scrape immediately.

### Bounded-blast-radius guarantees

- **Slow client / slow-loris:** `HTTP_READ_HEADER_TIMEOUT` (5s default)
  and `HTTP_READ_TIMEOUT` (15s default) cap how long a half-open
  request can hold a server worker.
- **Oversized payload:** Echo `BodyLimit("1M")` on every JSON write
  path; uploads cap separately via `MaxBytesReader` sized to
  `MAX_UPLOAD_SIZE_MB`.
- **Rate-limiter memory leak:** per-IP entries TTL out after 30 min
  idle (background sweeper); current population exposed via
  `rate_limit_active_clients`.

---

## Spec Compliance

| Category | Count | Status |
|----------|-------|--------|
| Endpoints (Household, Pickup, Payment, Reporting) | 16/16 | ✅ All implemented |
| Business rules (BR-01 .. BR-06) | 6/6 | ✅ All enforced |
| Technical requirements (TR-1 .. TR-6) | 6/6 | ✅ All met |
| Deliverables | 5/5 | ✅ All present |

Full per-requirement traceability (file paths + test names) is in [`plans/phase-7-final-verification.md`](plans/phase-7-final-verification.md).

### Business rules at a glance

| # | Rule | Enforcement |
|---|------|-------------|
| BR-01 | No new pickup if household has a pending payment | `internal/service/pickup.go` Create — advisory lock + partial UNIQUE index |
| BR-02 | Schedule only if status is `pending` | Conditional `UPDATE … WHERE status='pending'` → 409 on stale state |
| BR-03 | Electronic pickup: `safety_check` must be `true` before scheduling | Service + validator; 422 otherwise |
| BR-04 | Organic pickups auto-canceled after 3 days via goroutine | `internal/worker/organic_canceler.go` — ticks every minute, cleans up gracefully on shutdown |
| BR-05 | Completing a pickup auto-creates a payment (50 000 or 100 000) | `SELECT FOR UPDATE` + atomic tx in `internal/service/pickup.go` Complete |
| BR-06 | Payment confirmation requires S3 proof-of-payment file upload | MIME allowlist + magic-byte sniff + MinIO upload + URL saved to DB |

### Technical requirements at a glance

| # | Requirement | Where |
|---|-------------|-------|
| TR-1 | Dependency injection | Constructor wiring in `cmd/api/main.go`; interfaces in `internal/domain/` |
| TR-2 | Graceful shutdown | Signal handler + `wg.Wait()` in `cmd/api/main.go`; worker drains before exit |
| TR-3 | Rate limiting on pickup creation | Per-IP token bucket on `POST /api/pickups`; `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST` env vars |
| TR-4 | Docker + single command | `make docker-up` boots 9 services; `docker compose up` equivalently |
| TR-5 | Consistent API responses | Envelope `{success, data?, error?, meta?}` on every endpoint; audited in `error_envelope_test.go` |
| TR-6 | Input validation | `validator/v10` + custom `db_exists_household` / `db_exists_pickup` tags; tested per-field |

---

## Architecture Decisions

Eight ADRs document the load-bearing decisions in this codebase. They
live under [`docs/adr/`](docs/adr/) — each is a short MADR-style record
with context, decision, and consequences.

| # | Decision |
|---|----------|
| [0001](docs/adr/0001-no-orm.md) | No ORM — raw SQL via `sqlx` |
| [0002](docs/adr/0002-sentinel-errors.md) | Sentinel errors for domain outcomes |
| [0003](docs/adr/0003-shopspring-decimal.md) | `shopspring/decimal` for monetary amounts |
| [0004](docs/adr/0004-per-ip-rate-limit.md) | Per-IP token bucket rate limiting |
| [0005](docs/adr/0005-worker-context-cancellation.md) | Background worker with context cancellation |
| [0006](docs/adr/0006-business-rules-in-service-layer.md) | Business rules enforced in the service layer |
| [0007](docs/adr/0007-opentelemetry.md) | OpenTelemetry — vendor-neutral distributed tracing |
| [0008](docs/adr/0008-prometheus-red-metrics.md) | Prometheus + Grafana — RED metrics with auto-provisioning |
