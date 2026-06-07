[![CI](https://github.com/fairyhunter13/community-waste-collection-system/actions/workflows/ci.yml/badge.svg)](https://github.com/fairyhunter13/community-waste-collection-system/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/fairyhunter13/community-waste-collection-system/graph/badge.svg)](https://codecov.io/gh/fairyhunter13/community-waste-collection-system)

# Community Waste Collection API

A RESTful API service for managing community household waste collection,
pickup scheduling, and payment processing.

Built with Go 1.26, Echo v4, PostgreSQL 17, MinIO, and Docker.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Environment Variables](#environment-variables)
- [Running Locally (without Docker)](#running-locally-without-docker)
- [Running Tests](#running-tests)
- [API Reference](#api-reference)
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

Services started:

| Service | URL | Credentials |
|---|---|---|
| API | http://localhost:8080 | — |
| Grafana Dashboard | http://localhost:3000 | admin / admin |
| Prometheus | http://localhost:9090 | — |
| Jaeger UI (traces) | http://localhost:16686 | — |
| MinIO console | http://localhost:9001 | minioadmin / minioadmin |
| Prometheus metrics | http://localhost:2112/metrics | — |
| pprof debug | http://localhost:6060/debug/pprof/ | — |

The Grafana dashboard auto-provisions on startup and shows 6 rows of panels: API traffic, business events, database performance, background worker, Go runtime, and process metrics.

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
| `S3_ENDPOINT` | `http://localhost:9000` | S3-compatible storage endpoint |
| `S3_BUCKET` | `waste-proofs` | Bucket for payment proof uploads |
| `S3_ACCESS_KEY` | `minioadmin` | S3 access key |
| `S3_SECRET_KEY` | `minioadmin` | S3 secret key |
| `MAX_UPLOAD_SIZE_MB` | `10` | Maximum proof file upload size |
| `RATE_LIMIT_RPS` | `5` | Pickup creation rate limit (req/sec/IP) |
| `RATE_LIMIT_BURST` | `10` | Rate limit burst capacity |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | Log format: `json` or `text` |
| `METRICS_PORT` | `2112` | Prometheus metrics server port |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | OTel OTLP HTTP endpoint |
| `OTEL_SERVICE_NAME` | `community-waste-collection-api` | OTel service name |
| `WORKER_CANCEL_INTERVAL` | `1h` | How often the organic-canceler worker runs |
| `WORKER_ORGANIC_CUTOFF_DAYS` | `3` | Days after which a pending organic pickup is auto-cancelled |

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

## API Reference

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

**Rate limiting:** Pickup creation is rate-limited to 5 req/s per IP (burst 10).

### Payments

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/payments` | Create a payment record |
| `GET` | `/api/payments` | List payments (filterable by household, status) |
| `PUT` | `/api/payments/:id/confirm` | Confirm payment with proof file upload (multipart) |

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

List:
```json
{ "success": true, "data": [...], "meta": { "page": 1, "per_page": 20, "total": 5, "total_pages": 1 } }
```

Error:
```json
{ "success": false, "error": { "code": "NOT_FOUND", "message": "not found" } }
```

---

## Architecture

```
cmd/api/
  main.go             ← DI wiring + graceful shutdown

internal/
  config/             ← env-based configuration
  domain/             ← entities, interfaces, errors
  handler/            ← Echo HTTP handlers
  middleware/         ← rate limiter, logger, recover, OTel trace
  service/            ← business logic + business rules
  repository/         ← sqlx SQL implementations
  storage/            ← S3/MinIO upload client
  observability/      ← slog logger, OTel tracer, Prometheus metrics
  worker/             ← background organic-pickup auto-canceler
  mocks/              ← testify/mockery mocks (generated)

migrations/           ← golang-migrate SQL files
test/e2e/             ← end-to-end tests (build tag: e2e)
scripts/seed.sql      ← demo seed data
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
