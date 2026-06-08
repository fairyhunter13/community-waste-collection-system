# Requirements Fulfillment Matrix

Maps every stated requirement to the production-code path, the tests that
exercise it, and the CI job that verifies it on every push. Each row is an
auditable proof that the requirement is: (a) implemented, (b) tested, and
(c) continuously verified.

---

## A. Entities

### A1. Household

| Field | Type | Constraint | Schema | Domain struct |
|---|---|---|---|---|
| `id` | UUID | PK, auto-generated | `migrations/000001_create_households.up.sql:1` | `domain/household.go:13` |
| `owner_name` | TEXT | NOT NULL, min 1 char | `migrations/000001…:4` | `domain/household.go:14` |
| `address` | TEXT | NOT NULL, min 1 char | `migrations/000001…:5` | `domain/household.go:15` |
| `created_at` | TIMESTAMPTZ | NOT NULL, default NOW() | `migrations/000001…:6` | `domain/household.go:16` |
| `updated_at` | TIMESTAMPTZ | NOT NULL, default NOW() | `migrations/000001…:7` | `domain/household.go:17` |

**Unit tests:** `internal/service/household_test.go`  
**E2E tests:** `test/e2e/household_test.go`  
**CI job:** `test-unit`, `e2e`

---

### A2. Waste Pickup

| Field | Type | Constraint | Schema | Domain struct |
|---|---|---|---|---|
| `id` | UUID | PK, auto-generated | `migrations/000002_create_pickups.up.sql:8` | `domain/pickup.go:43` |
| `household_id` | UUID | FK → households, CASCADE | `migrations/000002…:9` | `domain/pickup.go:44` |
| `type` | ENUM | `organic\|plastic\|paper\|electronic` | `migrations/000002…:1-2,10` | `domain/pickup.go:17-22,45` |
| `status` | ENUM | `pending\|scheduled\|completed\|canceled` | `migrations/000002…:3-4,11` | `domain/pickup.go:24-30,46` |
| `pickup_date` | TIMESTAMPTZ | nullable | `migrations/000002…:12` | `domain/pickup.go:47` |
| `safety_check` | BOOLEAN | NOT NULL, default false; enforced for `electronic` by BR-03 | `migrations/000002…:13` | `domain/pickup.go:48` |
| `created_at` | TIMESTAMPTZ | NOT NULL, default NOW() | `migrations/000002…:14` | `domain/pickup.go:49` |
| `updated_at` | TIMESTAMPTZ | NOT NULL, default NOW() | `migrations/000002…:15` | `domain/pickup.go:50` |

**Unit tests:** `internal/service/pickup_test.go`  
**E2E tests:** `test/e2e/pickup_test.go`  
**CI job:** `test-unit`, `e2e`

---

### A3. Payment

| Field | Type | Constraint | Schema | Domain struct |
|---|---|---|---|---|
| `id` | UUID | PK, auto-generated | `migrations/000003_create_payments.up.sql:4` | `domain/payment.go:26` |
| `household_id` | UUID | FK → households, CASCADE | `migrations/000003…:5` | `domain/payment.go:27` |
| `waste_id` | UUID | FK → waste_pickups, UNIQUE, CASCADE | `migrations/000003…:6` | `domain/payment.go:28` |
| `amount` | NUMERIC(12,2) | NOT NULL, > 0, decimal-serialized | `migrations/000003…:7` | `domain/payment.go:29` |
| `payment_date` | TIMESTAMPTZ | nullable, set on confirmation | `migrations/000003…:8` | `domain/payment.go:30` |
| `status` | ENUM | `pending\|paid\|failed` | `migrations/000003…:1-2,9` | `domain/payment.go:14-19,31` |
| `proof_file_url` | TEXT | nullable, populated on confirmation | `migrations/000003…:10` | `domain/payment.go:32` |
| `created_at` | TIMESTAMPTZ | NOT NULL, default NOW() | `migrations/000003…:11` | `domain/payment.go:33` |
| `updated_at` | TIMESTAMPTZ | NOT NULL, default NOW() | `migrations/000003…:12` | `domain/payment.go:34` |

**Unit tests:** `internal/service/payment_test.go`  
**E2E tests:** `test/e2e/payment_test.go`  
**CI job:** `test-unit`, `e2e`

---

## B. Endpoints

All 15 endpoints are registered in `internal/handler/handler.go:169-188`.

| # | Method | Path | Handler | Unit test | E2E test |
|---|---|---|---|---|---|
| 1 | POST | `/api/households` | `CreateHousehold` (line 169) | `service/household_test.go:TestCreate*` | `e2e/household_test.go:TestHousehold_Create*` |
| 2 | GET | `/api/households` | `ListHouseholds` (line 170) | `service/household_test.go:TestList*` | `e2e/household_test.go:TestHousehold_List*` |
| 3 | GET | `/api/households/:id` | `GetHousehold` (line 171) | `service/household_test.go:TestGetByID*` | `e2e/household_test.go:TestHousehold_Get*` |
| 4 | DELETE | `/api/households/:id` | `DeleteHousehold` (line 172) | `service/household_test.go:TestDelete*` | `e2e/household_test.go:TestHousehold_Delete*` |
| 5 | POST | `/api/pickups` | `CreatePickup` (line 175) | `service/pickup_test.go:TestCreate*` | `e2e/pickup_test.go:TestPickup_Create*` |
| 6 | GET | `/api/pickups` | `ListPickups` (line 176) | `service/pickup_test.go:TestList*` | `e2e/pickup_test.go:TestPickup_List*` |
| 7 | PUT | `/api/pickups/:id/schedule` | `SchedulePickup` (line 177) | `service/pickup_test.go:TestSchedule*` | `e2e/pickup_test.go:TestPickup_Schedule*` |
| 8 | PUT | `/api/pickups/:id/complete` | `CompletePickup` (line 178) | `service/pickup_test.go:TestComplete*` | `e2e/pickup_test.go:TestPickup_Complete*` |
| 9 | PUT | `/api/pickups/:id/cancel` | `CancelPickup` (line 179) | `service/pickup_test.go:TestCancel*` | `e2e/pickup_test.go:TestPickup_Cancel*` |
| 10 | POST | `/api/payments` | `CreatePayment` (line 181) | `service/payment_test.go:TestCreate*` | `e2e/payment_test.go:TestPayment_Create*` |
| 11 | GET | `/api/payments` | `ListPayments` (line 182) | `service/payment_test.go:TestList*` | `e2e/payment_test.go:TestPayment_List*` |
| 12 | PUT | `/api/payments/:id/confirm` | `ConfirmPayment` (line 183) | `service/payment_test.go:TestConfirm*` | `e2e/payment_test.go:TestPayment_Confirm*` |
| 13 | GET | `/api/reports/waste-summary` | `WasteSummary` (line 186) | `service/report_test.go:TestWasteSummary*` | `e2e/report_test.go:TestReport_WasteSummary*` |
| 14 | GET | `/api/reports/payment-summary` | `PaymentSummary` (line 187) | `service/report_test.go:TestPaymentSummary*` | `e2e/report_test.go:TestReport_PaymentSummary*` |
| 15 | GET | `/api/reports/households/:id/history` | `HouseholdHistory` (line 188) | `service/report_test.go:TestHouseholdHistory*` | `e2e/report_test.go:TestReport_HouseholdHistory*` |

**API contract continuously verified:** `api/openapi.yaml` is linted against handler output by the `contract` CI job using Redocly (`api-contract-lint`).  
**Collections parity:** Postman collection: 27 requests. Insomnia collection: 27 requests. Both cover all 15 business endpoints plus reporting variants. Located at `api/community-waste.postman_collection.json` and `api/community-waste.insomnia_collection.json`.

---

## C. Business Rules

### BR-01 — Pending payment blocks new pickup

**Requirement:** A household cannot create a new pickup request if they have any payment with `pending` status.

| Layer | Location | What it does |
|---|---|---|
| Service | `service/pickup.go:40-60` (`Create`) | Calls `HasPendingPaymentForHousehold`; returns `ErrConflict` (→ HTTP 409) if `true` |
| Repository | `repository/pickup.go` (`HasPendingPaymentForHousehold`) | Queries the DB using the partial unique index |
| DB safety net | `migrations/000004_unique_open_payment.up.sql` | Partial unique index on `payments(household_id) WHERE status='pending'` — enforces at DB tier even under concurrent writes |

**Unit test:** `service/pickup_test.go:TestCreate_BlockedByPendingPayment`  
**E2E test:** `test/e2e/pickup_test.go:TestPickup_BlockedByPendingPayment`  
**Concurrency test:** `test/e2e/concurrency_test.go:TestConcurrent_BR01`  
**CI job:** `test-unit`, `e2e`

---

### BR-02 — Schedule requires `pending` status

**Requirement:** A pickup can only be scheduled if its current status is `pending`.

| Layer | Location | What it does |
|---|---|---|
| Service | `service/pickup.go:114-125` (`Schedule`) | Loads pickup, checks `status != pending`, returns `ErrConflict` |
| Repository | `repository/pickup.go` (`Schedule`) | Conditional `UPDATE … WHERE status = 'pending'`; `RowsAffected == 0` → conflict |

**Unit test:** `service/pickup_test.go:TestSchedule_RejectsNonPending`  
**E2E test:** `test/e2e/pickup_test.go:TestPickup_Schedule_AlreadyScheduled_409`  
**CI job:** `test-unit`, `e2e`

---

### BR-03 — Electronic pickup requires `safety_check = true` before scheduling

**Requirement:** Electronic type pickups cannot be scheduled unless `safety_check` is `true`.

| Layer | Location | What it does |
|---|---|---|
| Service | `service/pickup.go:127-136` (`Schedule`) | After BR-02 check: if `type == electronic && !safety_check` → `ErrBusinessRule` (→ HTTP 422) |

**Unit test:** `service/pickup_test.go:TestSchedule_Electronic_SafetyCheckRequired`  
**E2E test:** `test/e2e/pickup_test.go:TestPickup_Schedule_Electronic_NoSafetyCheck_422`  
**CI job:** `test-unit`, `e2e`

---

### BR-04 — Organic pickups auto-canceled after 3 days (background worker)

**Requirement:** Organic type pickups should be auto-canceled if not picked up within 3 days of creation. Implement as a background goroutine that shuts down cleanly on application exit.

| Layer | Location | What it does |
|---|---|---|
| Worker | `worker/organic_canceler.go:42-59` (`Start`) | `time.NewTicker` loop; exits on `ctx.Done()` for clean shutdown |
| Worker | `worker/organic_canceler.go:63-74` (`runWithRecover`) | Wraps each cycle in `recover()` — a panicking DB call cannot silently kill the goroutine |
| Worker | `worker/organic_canceler.go:77-131` (`run`) | Queries `FindExpiredOrganic(ctx, now-cutoff)`, calls `BulkCancel` for matched IDs |
| Repository | `repository/pickup.go` (`FindExpiredOrganic`) | `WHERE type='organic' AND status='pending' AND created_at < $1` — uses the partial index from migration 000002 |
| Goroutine lifecycle | `cmd/api/main.go:51, 107-114` | `startBackgroundWorker` launches via `sync.WaitGroup`; `workerCancel` context is cancelled on SIGTERM before `wg.Wait()` |
| Config | `config/config.go` | `WORKER_ORGANIC_CUTOFF_DAYS` (default 3), `WORKER_CANCEL_INTERVAL` |

**Unit tests:** `worker/organic_canceler_test.go` — 6 cases including cancel skip on error, empty result, BulkCancel error, context cancel, and `runWithRecover` panic recovery  
**Integration test:** `worker/organic_canceler_integration_test.go` (tag `integration`) — real Postgres via testcontainers; verifies 4-day-old row canceled, 1-day-old row untouched  
**CI job:** `test-unit`, `test-integration`

---

### BR-05 — Complete pickup atomically generates payment record

**Requirement:** Once a pickup is completed, automatically generate a payment record. `organic/plastic/paper → 50,000`. `electronic → 100,000`.

| Layer | Location | What it does |
|---|---|---|
| Domain amounts | `domain/pickup.go:33-38` (`PaymentAmounts`) | Canonical map: organic/plastic/paper = `50000.00`, electronic = `100000.00` |
| Service | `service/pickup.go:180-230` (`Complete`) | Opens `db.BeginTxx`, calls `UpdateStatus(… completed …)` + `paymentRepo.CreateWithTx`, then `Commit`; any step failure triggers `Rollback` |
| Status guard | `service/pickup.go:158-170` | Rejects non-`scheduled` pickups with `ErrConflict` |
| Atomic DB safety | `migrations/000003…:6` | `waste_id UNIQUE` on payments — second payment for same pickup raises `ErrConflict` at DB tier |

**Unit tests:** `service/pickup_test.go:TestComplete_*` — including `BeginTxFails`, `UpdateStatusInTxFails`, `CreateWithTxFails`, `CommitFails` branches  
**E2E test:** `test/e2e/pickup_test.go:TestPickup_Complete_*`, `test/e2e/payment_test.go:TestPayment_Amount_PerWasteType`  
**Concurrency test:** `test/e2e/concurrency_test.go:TestConcurrent_BR05`  
**CI job:** `test-unit`, `e2e`

---

### BR-06 — Payment confirmation requires S3 proof upload

**Requirement:** Payment confirmation requires uploading a proof of payment file to an S3-compatible storage service. The file URL must be saved to the payment record.

| Layer | Location | What it does |
|---|---|---|
| Handler | `handler/payment.go` (`ConfirmPayment`) | Parses multipart body; validates declared `Content-Type` against allowlist (`image/jpeg`, `image/png`, `application/pdf`); sniffs magic bytes; rejects oversized files |
| Service | `service/payment.go:86-130` (`Confirm`) | Uploads to MinIO via `StorageService.Upload`; writes URL to DB via `repo.Confirm`; on DB failure calls `storage.Delete` (best-effort S3 cleanup) |
| Storage | `storage/minio.go` | `Upload` generates a presigned URL; `Delete` for cleanup |
| DB | `domain/payment.go:32` | `proof_file_url *string` — nullable before confirmation, populated after |

**Unit tests:** `service/payment_test.go:TestConfirm_*` — including `FindByIDFails`, `RepoConfirmFails_DeletesUploadedObject`  
**E2E test:** `test/e2e/payment_test.go:TestPayment_Confirm_*`, `TestPayment_Confirm_StoresProofFileURL`  
**CI job:** `test-unit`, `e2e`

---

## D. Technical Requirements

### TR-1 — Dependency injection

**Requirement:** Use dependency injection throughout.

| Evidence | Location |
|---|---|
| All repositories constructed once, injected into services | `cmd/api/main.go:84-90` |
| All services constructed once, injected into handler | `cmd/api/main.go:88-95` |
| Interfaces in `domain` package; concrete impls in `repository/`, `service/`, `storage/` | `internal/domain/*.go` |
| Mock implementations generated by `mockery` | `internal/mocks/` |
| DB validators injected via `newValidator(db)` | `internal/handler/handler.go:26-65` |

---

### TR-2 — Graceful shutdown

**Requirement:** The application must shut down cleanly on exit.

| Evidence | Location |
|---|---|
| SIGINT/SIGTERM captured | `cmd/api/main.go:160-164` |
| Worker context cancelled before HTTP shutdown | `cmd/api/main.go:166-170` |
| `wg.Wait()` drains in-flight worker cycles with timeout | `cmd/api/main.go:183-193` |
| Echo HTTP server shutdown with configurable timeout | `cmd/api/main.go:194-199` |
| Metrics server shutdown | `cmd/api/main.go:201-204` |
| OTel tracer flushed | `cmd/api/main.go:206-209` |
| DB connection pool closed | `cmd/api/main.go:210-213` |

**Unit test:** `cmd/api/shutdown_test.go`  
**CI job:** `test-unit`

---

### TR-3 — Rate limiting on pickup creation

**Requirement:** Rate limiting on pickup creation (`POST /api/pickups`).

| Evidence | Location |
|---|---|
| `middleware.RateLimiter` applied only to `POST /api/pickups` | `handler/handler.go:175` |
| Per-IP token bucket using `golang.org/x/time/rate` in `sync.Map` | `middleware/ratelimit.go:38-115` |
| Configurable via `RATE_LIMIT_RPS` and `RATE_LIMIT_BURST` env vars | `config/config.go` |
| Returns HTTP 429 with envelope on limit exceeded | `middleware/ratelimit.go` |
| Idle client eviction to prevent unbounded map growth | `middleware/ratelimit.go:97-115` |

**Unit test:** `middleware/ratelimit_test.go`  
**E2E test:** `test/e2e/ratelimit_test.go`  
**CI job:** `test-unit`, `e2e`

---

### TR-4 — Docker with single-command startup

**Requirement:** Docker (app + PostgreSQL), single command to run.

| Evidence | Location |
|---|---|
| `docker compose -f deployments/docker-compose.yml up --build` starts all services | `deployments/docker-compose.yml` |
| Services: API, PostgreSQL 17, MinIO, Jaeger, Loki, Promtail, Grafana | `deployments/docker-compose.yml` |
| Multi-stage Dockerfile: `golang:1.26-alpine` builder → `alpine:3.21` runtime | `build/Dockerfile` |
| `Makefile` provides `make up`, `make down`, `make migrate-up` shortcuts | `Makefile` |
| `.env.example` documents all required variables; copied to `.env` on first run | `.env.example` |

---

### TR-5 — Consistent API responses with appropriate HTTP status codes

**Requirement:** Consistent API responses with appropriate HTTP status codes.

| HTTP status | Trigger | Envelope shape |
|---|---|---|
| 200 | Successful GET / PUT | `{"success":true,"data":{...},"meta":{...}}` |
| 201 | Successful POST (resource created) | `{"success":true,"data":{...}}` |
| 400 | Validation error | `{"success":false,"error":{"code":"validation_error","message":"..."}}` |
| 404 | Resource not found | `{"success":false,"error":{"code":"NOT_FOUND","message":"..."}}` |
| 405 | Method not allowed | `{"success":false,"error":{"code":"METHOD_NOT_ALLOWED","message":"..."}}` |
| 409 | Business-rule conflict (BR-01, BR-02, BR-05) | `{"success":false,"error":{"code":"CONFLICT","message":"..."}}` |
| 422 | Business rule violation (BR-03) | `{"success":false,"error":{"code":"BUSINESS_RULE_VIOLATION","message":"..."}}` |
| 429 | Rate limit exceeded (TR-3) | `{"success":false,"error":{"code":"TOO_MANY_REQUESTS","message":"..."}}` |
| 500 | Internal error | `{"success":false,"error":{"code":"INTERNAL_ERROR","message":"..."}}` |

**Implementation:** `handler/handler.go:echoErrorHandler` (registered via `e.HTTPErrorHandler`)  
**Unit tests:** `handler/misc_test.go:TestEchoErrorHandler_*`  
**OpenAPI spec:** `api/openapi.yaml` — all response shapes documented and linted by `api-contract-lint` CI job

---

### TR-6 — Input validation

**Requirement:** Input validation on all endpoints.

| Validation type | Implementation | Location |
|---|---|---|
| Struct-tag validation (`required`, `min`, `oneof`) | `go-playground/validator/v10` | `domain/*.go` validate tags |
| UUID format | `google/uuid` binding via Echo | `handler/*.go` `c.Param("id")` |
| Enum binding (waste type, pickup status, payment status) | `oneof=…` validator tag | `domain/pickup.go:57`, `handler/pickup.go` |
| DB-existence check (household_id, waste_id) | Custom `db_exists_*` validators | `handler/handler.go:34-58` |
| Positive decimal | Custom `positive_decimal` validator | `handler/handler.go:28-31` |
| Pagination bounds (`page >= 1`, `per_page <= 100`) | `paginationParams` helper | `handler/handler.go:326-350` |
| Proof file MIME type + magic-byte sniff | Allowlist check + `http.DetectContentType` | `handler/payment.go` (`ConfirmPayment`) |
| Proof file size limit | `echo.BodyLimit` + multipart max | `handler/handler.go` |

**Unit tests:** `handler/misc_test.go:TestPaginationParams_*`; `service/*_test.go` validation path tests  
**E2E tests:** `test/e2e/pickup_test.go:TestPickup_InvalidWasteType_400`, `TestPickup_InvalidStatusFilter_400`  
**CI job:** `test-unit`, `e2e`

---

## E. Deliverables

| # | Deliverable | Status | Location |
|---|---|---|---|
| 1 | Go project with PostgreSQL + Docker | ✅ | `build/Dockerfile`, `deployments/docker-compose.yml`; `make up` starts everything |
| 2 | Source code with documented structure | ✅ | `internal/` (domain / handler / service / repository / middleware / worker / storage / observability / config); architecture in `docs/architecture.md` |
| 3 | Postman + Insomnia collections covering all endpoints | ✅ | `api/community-waste.postman_collection.json` (27 requests), `api/community-waste.insomnia_collection.json` (27 requests) |
| 4 | README.md | ✅ | `README.md` — see breakdown below |
| 5 | GitHub repository with daily commits throughout the development window | ✅ | 102 commits across the development window (2026-06-05 → 2026-06-09) |

### Deliverable 4 — README.md sub-items

| README requirement | Section | Line range |
|---|---|---|
| Setup and run instructions | `## Quick Start` | ~72 |
| How to run migrations | `make migrate-up` in Quick Start + `## Troubleshooting` | ~77, ~123 |
| Seeding | `## Running Locally` step 3 (`make seed`) | ~297 |
| Environment variable reference | `## Environment Variables` | ~254 |
| Architecture decisions | `## Architecture Decisions` | ~785 |

---

## F. CI Pipeline

All 9 jobs in `.github/workflows/ci.yml` run on every push to `main`.

| Job | Runs | What it covers |
|---|---|---|
| `lint` | `golangci-lint` | Code quality: `errcheck`, `staticcheck`, `govet`, `unused`, `gosec` |
| `test-unit` | `go test ./internal/...` | All unit tests; coverage gate `> 80 %` (current: **87.1 %**) |
| `test-integration` | `go test -tags=integration ./internal/...` | Repository + worker tests against real Postgres (testcontainers) |
| `coverage-gate` | `go tool cover` threshold check | Ensures coverage never drops below the gate |
| `e2e` | `go test -tags=e2e ./test/e2e/...` | Full end-to-end suite (65 tests) against live Docker Compose stack |
| `api-contract-lint` | Redocly lint + contract check | `api/openapi.yaml` validity; Postman/Insomnia parity |
| `image-scan` | `aquasecurity/trivy-action@v0.36.0` | Container image HIGH/CRITICAL CVE scan |
| `vuln-scan` | `govulncheck` | Go module vulnerability database scan |
| `perf` | k6 load test | Throughput baseline for `POST /api/pickups` |

---

## G. Evaluation-Criteria Mapping

| Criterion | Evidence summary |
|---|---|
| **Correctness** | All 6 business rules implemented at service layer + DB constraint tier; 65 E2E tests exercise the golden path and all rejection branches; concurrency tests (BR-01, BR-05) verify race-safe enforcement |
| **Code quality** | Idiomatic Go: interfaces in `domain`, concrete impls injected; `slog` structured logging; OTel traces on every service call; `golangci-lint` with `errcheck`/`staticcheck`/`gosec` enforced in CI; 87.1 % unit coverage |
| **Architecture** | Strict layering: `cmd/api → handler → service → repository → PostgreSQL`; no import cycles (`overview import_cycles` returns clean); DI at `main.go:84-95`; documented in `docs/architecture.md` with 3 visual diagrams |
| **API design** | RESTful resource paths; consistent `{success, data, meta, error}` envelope; correct HTTP verbs and status codes (200/201/400/404/405/409/422/429/500); OpenAPI spec linted on every push |
| **Production readiness** | Graceful shutdown (SIGTERM → worker drain → HTTP shutdown → tracer flush → DB close); per-IP token-bucket rate limiting; Prometheus metrics + OTel traces + structured logs → Grafana/Jaeger/Loki; multi-stage Docker image; testcontainers integration suite |
| **Documentation** | `README.md` covers setup, migrations, seeding, env vars, architecture decisions; `docs/` knowledge base with 7 documents and 16 mermaid diagrams; OpenAPI spec; Postman + Insomnia collections |
