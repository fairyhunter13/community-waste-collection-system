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
