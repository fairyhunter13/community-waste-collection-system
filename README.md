[![CI](https://github.com/fairyhunter13/community-waste-collection-system/actions/workflows/ci.yml/badge.svg)](https://github.com/fairyhunter13/community-waste-collection-system/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/fairyhunter13/community-waste-collection-system/graph/badge.svg)](https://codecov.io/gh/fairyhunter13/community-waste-collection-system)
[![Coverage](https://img.shields.io/codecov/c/github/fairyhunter13/community-waste-collection-system/main)](https://codecov.io/gh/fairyhunter13/community-waste-collection-system)

# Community Waste Collection API

A RESTful API service for managing community household waste collection,
pickup scheduling, and proof-of-payment processing.

Built with Go 1.26, Echo v4, PostgreSQL 17, MinIO, and Docker Compose.

---

## Key Features

- **15 product REST endpoints** across households, pickups, payments, and reports plus 3 operational endpoints (`/health`, `/readyz`, `/metrics`)
- **6 business rules** enforced in the service layer:
  - BR-01 — A household with any pending payment cannot create a new pickup
  - BR-02 — Only pending pickups can be scheduled; only scheduled can be completed or cancelled
  - BR-03 — Electronic waste pickup requires a `safety_check: true` flag
  - BR-04 — Organic pickups with no scheduled date for 3 days are auto-cancelled by a background worker
  - BR-05 — Completing a pickup atomically auto-generates a payment record at the confirmed amount
  - BR-06 — Payment confirmation requires a multipart proof-of-payment file upload
- **Per-IP rate limiting** on pickup creation (5 req/s, burst 10) via token bucket
- **Full-stack observability**: structured JSON logs (slog), distributed tracing (OTel → Jaeger), 21 Prometheus instruments, 3 auto-provisioned Grafana dashboards
- **Unit test coverage ≥80%** enforced in CI; integration tests use real PostgreSQL via testcontainers
- **OpenAPI 3.0 spec** documented in `api/openapi.yaml`

---

## Documentation

| Doc | Contents |
|---|---|
| [docs/architecture.md](docs/architecture.md) | Layers, request flow, business processes, data model, decisions |
| [docs/deployment.md](docs/deployment.md) | Compose topology, observability data flow, graceful shutdown |
| [docs/operations.md](docs/operations.md) | Failure-mode decision tree, health endpoints, recovery |
| [api/openapi.yaml](api/openapi.yaml) | The endpoint contract, error envelope and status codes |

---

## Prerequisites

- **Go 1.26+** (`go version` should report `go1.26.x`).
- **Docker 24+** with the **Compose v2 plugin** (`docker compose version` should report `2.x`). The legacy `docker-compose` shim is not used.
- **GNU make** (`make --version`). On macOS, ship Xcode Command Line Tools.
- **Ports free** on the host: `8080` (API), `5432` (Postgres), `9000`/`9001` (MinIO), `3000` (Grafana), `9090` (Prometheus), `16686` (Jaeger UI), `4317`/`4318` (Jaeger OTLP), `3100` (Loki), `2112` (Prometheus scrape target), `6060` (pprof). See [Troubleshooting](#troubleshooting) if any of these collide with a host service.
- **Optional but recommended for local SQL work:**
  - [`migrate` CLI](https://github.com/golang-migrate/migrate/releases) — run migrations from the host without entering the API container.
  - `psql` client — inspect the live DB during development.
  - [`newman`](https://github.com/postmanlabs/newman) (`npm i -g newman`) — replay the Postman collection against a running stack.

---

## Quick Start

```bash
cp .env.example .env
make docker-up       # start all services (postgres, minio, jaeger, prometheus, grafana, loki, promtail, api)
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
| Grafana Dashboard | http://localhost:3000 | admin / admin |
| Prometheus | http://localhost:9090 | — |
| Jaeger UI (traces) | http://localhost:16686 | — |
| MinIO console | http://localhost:9001 | minioadmin / minioadmin |
| Prometheus metrics | http://localhost:2112/metrics | — |
| pprof debug | http://localhost:6060/debug/pprof/ | — |

Grafana auto-provisions three dashboards on startup:

- **Waste Collection API** — 7 rows: API traffic, business events, database performance, background worker, Go runtime, process metrics, S3 storage, and Jaeger traces
- **Business Operations** — 4 rows: pickup funnel, payment funnel, error breakdown, S3 storage KPIs
- **Logs & Traces** — Loki log stream with trace-ID correlation links to Jaeger

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
  -d '{"pickup_date":"2026-02-14T09:00:00Z"}' | jq .
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
| `DB_CONN_MAX_LIFETIME` | `30m` | Max lifetime of a DB connection before it is recycled |
| `DB_APPLICATION_NAME` | `waste-collection-api` | `application_name` sent to Postgres (visible in `pg_stat_activity`) |
| `S3_ENDPOINT` | `http://localhost:9000` | S3-compatible storage endpoint |
| `S3_BUCKET` | `waste-proofs` | Bucket for payment proof uploads |
| `S3_ACCESS_KEY` | `minioadmin` | S3 access key |
| `S3_SECRET_KEY` | `minioadmin` | S3 secret key |
| `S3_REGION` | `us-east-1` | S3 region (required by AWS SDK; MinIO ignores it) |
| `S3_USE_PATH_STYLE` | `true` | Use path-style S3 URLs (required for MinIO) |
| `MAX_UPLOAD_SIZE_MB` | `10` | Maximum proof file upload size |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | Max time to read request headers |
| `HTTP_READ_TIMEOUT` | `15s` | Max time to read the full request (headers + body) |
| `HTTP_WRITE_TIMEOUT` | `15s` | Max time to write the response |
| `HTTP_IDLE_TIMEOUT` | `60s` | Max time a keep-alive connection can be idle |
| `HTTP_SHUTDOWN_TIMEOUT` | `15s` | Grace period for in-flight requests on shutdown |
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
| `WORKER_QUERY_TIMEOUT` | `5s` | Max time for a single DB query inside the worker |
| `WORKER_SHUTDOWN_TIMEOUT` | `30s` | Grace period for the worker to finish its current cycle on shutdown |
| `CODECOV_TOKEN` | — | Codecov upload token (CI only, never commit the value) |

See `.env.example` for a complete reference with comments.

---

## Running Locally (without Docker)

```bash
# 1. Start infrastructure only
docker compose up -d postgres minio jaeger

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

## Known Limitations

- **DB and MinIO credentials default to development values.** Override `DATABASE_URL`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` via environment variables or a secrets manager before deploying to production.
- **Jaeger uses in-memory trace storage in docker-compose.** Spans are lost on container restart — intentional for local development. Use a persistent backend (e.g. Elasticsearch) in production.
- **`DELETE /api/households` performs a hard cascade delete.** The household and all linked pickups/payments are permanently removed. Audit trail preservation is out of scope for v1.
- **`WORKER_CANCEL_INTERVAL` adds up to one tick of drift to the 3-day organic-cancellation cutoff.** At the default 1-hour interval the worst case is ~73 hours. Reduce `WORKER_CANCEL_INTERVAL` for tighter SLAs.
- **pprof debug server binds to `127.0.0.1` only.** Not reachable from outside the container; no host port is mapped.

---

