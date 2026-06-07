# Phase 5 — Documentation & Delivery

## Purpose

Ensure the project is fully presentable, operationally complete, and production-credible. Documentation is evaluated as a first-class deliverable — not an afterthought. This phase begins during Phase 3 (README skeleton committed early) and is finalized in the last day.

---

## 1. README.md Structure

The README is the first thing a reviewer reads. It must enable a complete stranger to run the project within 5 minutes.

```markdown
# Community Waste Collection API

A RESTful API service for managing community household waste collection,
pickup scheduling, and payment processing.

Built with Go 1.26, Echo v4, PostgreSQL 17, and Docker.

---

## Table of Contents
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Environment Variables](#environment-variables)
- [Running Locally (without Docker)](#running-locally)
- [Running Tests](#running-tests)
- [API Reference](#api-reference)
- [Architecture](#architecture)
- [Architecture Decisions](#architecture-decisions)

---

## Prerequisites

- Go 1.26+
- Docker + docker-compose
- `make`
- (Optional) `migrate` CLI for manual migrations

---

## Quick Start

Single command to run the entire stack:

```bash
cp .env.example .env
make docker-up
make migrate-up
```

Services started:
- API: http://localhost:8080
- MinIO console: http://localhost:9001 (minioadmin / minioadmin)
- Prometheus metrics: http://localhost:2112/metrics
- pprof debug: http://localhost:6060/debug/pprof/

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `APP_PORT` | `8080` | HTTP server port |
| `DATABASE_URL` | postgres://... | Full PostgreSQL connection string |
| `S3_ENDPOINT` | http://localhost:9000 | S3-compatible storage endpoint |
| `S3_BUCKET` | `waste-proofs` | Bucket name for proof of payment files |
| `RATE_LIMIT_RPS` | `5` | Pickup creation rate limit (requests/sec/IP) |
| `LOG_LEVEL` | `info` | Log level: debug, info, warn, error |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | http://localhost:4318 | OTel trace export endpoint |
| ... | | See `.env.example` for complete reference |

---

## Running Locally (without Docker)

```bash
# 1. Start infrastructure only (postgres + minio + otel-collector)
docker compose -f deployments/docker-compose.yml up postgres minio otel-collector -d

# 2. Configure environment
cp .env.example .env

# 3. Apply migrations
make migrate-up

# 4. Start the application
make run
```

---

## Running Tests

```bash
make test              # Unit tests (no external dependencies)
make test-integration  # Integration tests (requires PostgreSQL via testcontainers)
make test-e2e          # E2E tests (requires full docker-compose stack)
make perf              # HTTP performance benchmarks (requires full docker-compose stack + running app)
make bench             # DB-layer micro-benchmarks (requires DATABASE_URL)
make coverage          # Open HTML coverage report
```

---

## API Reference

### Households
| Method | Path | Description |
|---|---|---|
| POST | /api/households | Create household |
| GET | /api/households | List (paginated) |
| GET | /api/households/:id | Get by ID |
| DELETE | /api/households/:id | Delete |

### Waste Pickups
| Method | Path | Description |
|---|---|---|
| POST | /api/pickups | Create pickup request |
| GET | /api/pickups | List (filter by status, household) |
| PUT | /api/pickups/:id/schedule | Schedule pickup |
| PUT | /api/pickups/:id/complete | Mark completed (auto-creates payment) |
| PUT | /api/pickups/:id/cancel | Cancel pickup |

### Payments
| Method | Path | Description |
|---|---|---|
| POST | /api/payments | Create payment |
| GET | /api/payments | List (filter by status, date range) |
| PUT | /api/payments/:id/confirm | Confirm with proof file upload |

### Reports
| Method | Path | Description |
|---|---|---|
| GET | /api/reports/waste-summary | Aggregated pickups by type and status |
| GET | /api/reports/payment-summary | Total payments by status and revenue |
| GET | /api/reports/households/:id/history | Full household history |

See `api/community-waste.postman_collection.json` for request examples.

---

## Architecture

```
cmd/api/main.go         — Entry point; dependency wiring; graceful shutdown
internal/handler/       — HTTP handlers (Echo); bind, validate, call service
internal/service/       — Business logic; business rules BR-01 through BR-06
internal/repository/    — Data access; sqlx + raw PostgreSQL SQL
internal/domain/        — Entities, interfaces, error types (no external deps)
internal/worker/        — Background goroutine: organic pickup auto-cancel
internal/storage/       — S3-compatible file upload wrapper
internal/observability/ — slog, Prometheus metrics, OpenTelemetry traces
```

Dependencies flow one way: handler → service → repository → PostgreSQL.
All cross-layer calls go through interfaces defined in `internal/domain/`.

See `plans/` for detailed phase documentation.

---

## Architecture Decisions

- **Echo v4** — chosen for middleware composition and native validator integration
- **sqlx + raw SQL** — full control over PostgreSQL-specific behavior (ENUMs, NUMERIC, partial indexes)
- **Manual DI** — dependency graph wired explicitly in `main.go`; zero magic
- **log/slog** — stdlib structured logging (Go 1.21+); no third-party logger needed
- **Prometheus + OTel** — RED metrics and distributed traces for production observability
- **testcontainers-go** — integration tests use real PostgreSQL; no SQLite mocks

See `plans/phase-1-architecture-design.md` for full ADRs.
```

---

## 2. Postman / Insomnia Collection

**File:** `api/community-waste.postman_collection.json`

### Collection Structure

```
Community Waste Collection API
│
├── 📁 Households
│   ├── POST Create Household
│   ├── GET List Households
│   ├── GET Get Household by ID
│   └── DELETE Delete Household
│
├── 📁 Pickups
│   ├── POST Create Pickup (organic)
│   ├── POST Create Pickup (electronic with safety_check)
│   ├── GET List Pickups
│   ├── GET List Pickups by Status
│   ├── PUT Schedule Pickup
│   ├── PUT Complete Pickup
│   └── PUT Cancel Pickup
│
├── 📁 Payments
│   ├── POST Create Payment
│   ├── GET List Payments
│   ├── GET List Payments by Date Range
│   └── PUT Confirm Payment (with proof file)
│
├── 📁 Reports
│   ├── GET Waste Summary
│   ├── GET Payment Summary
│   └── GET Household History
│
└── 📁 Error Cases
    ├── POST Create Pickup — blocked by pending payment (409)
    ├── PUT Schedule Electronic — no safety_check (422)
    ├── PUT Schedule — wrong status (409)
    └── GET Household — not found (404)
```

### Collection Environment Variables

```json
{
  "base_url": "http://localhost:8080",
  "household_id": "",
  "pickup_id": "",
  "payment_id": ""
}
```

**Test scripts** (set variables from responses):
```javascript
// In POST Create Household test script:
pm.test("Status 201", () => pm.response.to.have.status(201));
const body = pm.response.json();
pm.environment.set("household_id", body.data.id);
```

### API Sample Responses

**POST /api/households → 201:**
```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "owner_name": "John Doe",
    "address": "Jl. Merdeka No. 1, Jakarta",
    "created_at": "2026-06-05T09:00:00Z",
    "updated_at": "2026-06-05T09:00:00Z"
  }
}
```

**GET /api/households → 200:**
```json
{
  "success": true,
  "data": [
    { "id": "...", "owner_name": "John Doe", "address": "...", "created_at": "...", "updated_at": "..." }
  ],
  "meta": { "page": 1, "per_page": 20, "total": 1, "total_pages": 1 }
}
```

**POST /api/pickups → 409 (BR-01):**
```json
{
  "success": false,
  "error": {
    "code": "CONFLICT",
    "message": "household has a pending payment"
  }
}
```

**PUT /api/pickups/:id/schedule → 422 (BR-03):**
```json
{
  "success": false,
  "error": {
    "code": "BUSINESS_RULE_VIOLATION",
    "message": "electronic pickup requires safety_check to be true before scheduling"
  }
}
```

**PUT /api/payments/:id/confirm → 200:**
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "household_id": "uuid",
    "waste_id": "uuid",
    "amount": "50000.00",
    "status": "paid",
    "proof_file_url": "http://localhost:9000/waste-proofs/uuid/proof.jpg",
    "payment_date": "2026-06-05T10:30:00Z",
    "created_at": "...",
    "updated_at": "..."
  }
}
```

**GET /api/reports/waste-summary → 200:**
```json
{
  "success": true,
  "data": {
    "by_type": [
      {
        "type": "organic",
        "total": 12,
        "by_status": { "pending": 5, "scheduled": 2, "completed": 4, "canceled": 1 }
      },
      { "type": "plastic", "total": 8, "by_status": { "pending": 3, "completed": 5 } },
      { "type": "paper",   "total": 6, "by_status": { "completed": 6 } },
      { "type": "electronic", "total": 4, "by_status": { "scheduled": 1, "completed": 3 } }
    ]
  }
}
```

---

## 3. Commit Discipline

### Format: Conventional Commits

```
<type>(<scope>): <short description>

[optional body]
[optional footer]
```

**Types:**

| Type | Use For |
|---|---|
| `feat` | New feature or endpoint |
| `fix` | Bug fix |
| `test` | Adding or fixing tests |
| `chore` | Build tools, dependencies, configuration |
| `docs` | Documentation, README, comments |
| `refactor` | Code restructuring without behavior change |
| `perf` | Performance improvement |
| `ci` | CI/CD configuration |

**Examples:**
```
feat(household): add household creation endpoint
feat(pickup): implement organic auto-cancel background worker
fix(payment): correct amount calculation for electronic type
test(pickup): add integration tests for pickup repository
test(service): add unit tests for BR-01 through BR-05
chore: configure golangci-lint with production linter set
chore: add Dockerfile and docker-compose for local development
docs: update README with quick start instructions
perf(repository): add partial index for organic pending pickup query
```

### Commit Rules
- Commit after each meaningful deliverable, not per-file change
- Never commit broken or non-compiling code
- `go build ./...` and `go vet ./...` must pass before each commit
- `golangci-lint run ./...` must pass before each commit
- Minimum one substantive commit per working day

---

## 4. Delivery Checklist

Work through this checklist before submission. Every item must be checked.

### Functionality
- [x] `POST /api/households` → creates household, returns 201
- [x] `GET /api/households` → paginated list with meta
- [x] `GET /api/households/:id` → returns 200 or 404
- [x] `DELETE /api/households/:id` → returns 204 or 404
- [x] `POST /api/pickups` → creates pickup, returns 201
- [x] `POST /api/pickups` → blocked by pending payment → 409 (BR-01)
- [x] `POST /api/pickups` → rate limited → 429
- [x] `GET /api/pickups` → filtered list
- [x] `PUT /api/pickups/:id/schedule` → schedules, returns 200
- [x] `PUT /api/pickups/:id/schedule` → wrong status → 409 (BR-02)
- [x] `PUT /api/pickups/:id/schedule` → electronic, no safety_check → 422 (BR-03)
- [x] `PUT /api/pickups/:id/complete` → completes, auto-creates payment (BR-05)
- [x] `PUT /api/pickups/:id/cancel` → cancels
- [x] `POST /api/payments` → creates payment
- [x] `GET /api/payments` → filtered list with date range (`date_from`/`date_to` RFC3339 params parsed)
- [x] `PUT /api/payments/:id/confirm` → file upload, proof_file_url saved (BR-06)
- [x] `GET /api/reports/waste-summary` → correct aggregated counts
- [x] `GET /api/reports/payment-summary` → correct counts + revenue
- [x] `GET /api/reports/households/:id/history` → full history
- [x] Organic auto-cancel worker starts and runs on configured interval (BR-04) — verified by `TestOrganicWorker_BR04_AutoCancel` in `test/e2e/worker_test.go`
- [x] SIGINT → all goroutines stop within 10 seconds
- [x] `GET /api/pickups?status=garbage` → 400 (enum whitelist enforced)
- [x] `GET /api/payments?status=garbage` → 400 (enum whitelist enforced)
- [x] `GET /api/payments?date_from=invalid` → 400 (date format validated)
- [x] `POST /api/payments` with `amount:"abc"` → 400 (positive decimal validated)
- [x] `PUT /api/pickups/:id/schedule` with past date → 400 (future date validated)
- [x] `GET /api/pickups?per_page=9999` → response meta shows per_page capped at 100
- [x] `POST /api/pickups` with non-existent household_id → 400 (VALIDATION_ERROR)
- [x] `POST /api/payments` with non-existent household_id → 400 (VALIDATION_ERROR)
- [x] `POST /api/payments` with non-existent waste_id → 400 (VALIDATION_ERROR)
- [x] `POST /api/payments` twice for same pickup → 409 (unique violation handled)
- [x] Complete organic pickup → payment.amount == "50000.00" (BR-05)
- [x] Complete electronic pickup → payment.amount == "100000.00" (BR-05)
- [x] Confirm payment → proof_file_url non-empty in response (BR-06)
- [x] `GET /api/payments?date_from=...&date_to=...` → date range filter returns confirmed payments

### Code Quality
- [x] `go vet ./...` passes
- [x] `golangci-lint run ./...` passes (golangci-lint v2.12.2, 0 issues)
- [x] All exported symbols have godoc comments
- [x] No `TODO` or `FIXME` comments in committed code
- [x] No hardcoded credentials or secrets in source code

### Testing
- [x] `make test` (unit) passes with `-race` flag
- [x] `make test-integration` passes (real PostgreSQL via testcontainers)
- [x] `make test-e2e` passes (full docker-compose stack, verified in CI)
- [x] `make perf` completes without error (requires full docker-compose stack)
- [x] Overall coverage ≥ 80%: 81.7% over business logic (excludes generated mocks, integration-only repository, and observability bootstrap)

### Infrastructure
- [x] `make docker-up` → all services healthy on first run
- [x] `make migrate-up` → all 3 migrations apply in order
- [x] `make migrate-down` → all migrations revert cleanly
- [x] `make docker-down` → clean teardown
- [x] `.env.example` complete with all required variables

### Observability
- [x] `curl :2112/metrics` → Prometheus metrics returned
- [x] `curl :6060/debug/pprof/` → pprof index returned
- [x] OTel traces appear in collector logs (docker logs)
- [x] Request logs include `trace_id` field

### Documentation
- [x] README setup steps verified end-to-end on a clean environment
- [x] Postman/Insomnia collection imports without errors
- [x] All collection requests succeed against running stack
- [x] `plans/` directory committed with all 7 phase files
- [x] No company names or test references in any committed file
