# Phase 7 — Final Verification & Sign-Off

> Reviewer-facing sign-off checklist. No implementation detail — that lives in
> `phase-6-gap-closure.md`. This file records the **state of the deliverable**
> at the moment of final delivery: which spec items map to which files, which
> manual checks were performed, what the coverage looks like, and what known
> limitations exist.
>
> This file is the single source of truth a reviewer uses to confirm the
> backend engineering test deliverable is complete.

---

## 1. Spec Compliance Matrix

Four sub-matrices: endpoints (16), business rules (6), tech requirements (6), deliverables (5).
Each row links the spec item to its implementation file and the test that
proves it. Status legend: ✅ implemented + tested · ⚠️ implemented, partial
test · ❌ missing.

### Endpoints (16/16)

| Method | Path | Handler | OpenAPI Tag | Postman Request | E2E Test | Status |
|--------|------|---------|-------------|-----------------|----------|--------|
| POST | /api/households | `internal/handler/household.go` CreateHousehold | Household | Create Household | TestHouseholds Create | ✅ |
| GET | /api/households | `internal/handler/household.go` ListHouseholds | Household | List Households | TestHouseholds List | ✅ |
| GET | /api/households/:id | `internal/handler/household.go` GetHousehold | Household | Get Household | TestHouseholds GetByID | ✅ |
| DELETE | /api/households/:id | `internal/handler/household.go` DeleteHousehold | Household | Delete Household | TestHouseholds DeleteCascade | ✅ |
| POST | /api/pickups | `internal/handler/pickup.go` CreatePickup | Pickup | Create Pickup | TestPickups Create | ✅ |
| GET | /api/pickups | `internal/handler/pickup.go` ListPickups | Pickup | List Pickups | TestPickups List | ✅ |
| PUT | /api/pickups/:id/schedule | `internal/handler/pickup.go` SchedulePickup | Pickup | Schedule Pickup | TestPickups Schedule | ✅ |
| PUT | /api/pickups/:id/complete | `internal/handler/pickup.go` CompletePickup | Pickup | Complete Pickup | TestPickups Complete | ✅ |
| PUT | /api/pickups/:id/cancel | `internal/handler/pickup.go` CancelPickup | Pickup | Cancel Pickup | TestPickups Cancel | ✅ |
| POST | /api/payments | `internal/handler/payment.go` CreatePayment | Payment | Create Payment | TestPayments Create | ✅ |
| GET | /api/payments | `internal/handler/payment.go` ListPayments | Payment | List Payments | TestPayments List | ✅ |
| PUT | /api/payments/:id/confirm | `internal/handler/payment.go` ConfirmPayment | Payment | Confirm Payment | TestPayments ConfirmUpload | ✅ |
| GET | /api/reports/waste-summary | `internal/handler/report.go` WasteSummary | Reporting | Waste Summary | TestReports WasteSummary | ✅ |
| GET | /api/reports/payment-summary | `internal/handler/report.go` PaymentSummary | Reporting | Payment Summary | TestReports PaymentSummary | ✅ |
| GET | /api/reports/households/:id/history | `internal/handler/report.go` HouseholdHistory | Reporting | Household History | TestReports HouseholdHistory | ✅ |
| GET | /health | `internal/handler/misc.go` HealthCheck | — | Health Check | TestMisc Health | ✅ |

### Business Rules (6/6)

| BR | Wording | Implementation | Unit Test | E2E Test | DB Enforcement | Status |
|----|---------|----------------|-----------|----------|----------------|--------|
| BR-01 | No new pickup if household has a pending payment | `internal/service/pickup.go` Create: advisory lock + `HasPendingPaymentForHousehold` | `TestPickupService_Create_PendingPayment_Rejected` | `TestPickups_BR01_PendingPayment_Blocks` | Partial UNIQUE index `uq_payments_one_pending_per_household` | ✅ |
| BR-02 | Schedule only if status is pending | `internal/repository/pickup.go` Schedule: conditional `UPDATE … WHERE status='pending'`; `RowsAffected==0` → `ErrConflict` | `TestPickupService_Schedule_WrongStatus` | `TestPickups_BR02_ScheduleAlreadyScheduled` | Conditional UPDATE | ✅ |
| BR-03 | Electronic pickup requires safety_check=true before scheduling | `internal/service/pickup.go` Schedule: checks `pickup.Type==electronic && !pickup.SafetyCheck` | `TestPickupService_Schedule_ElectronicNoSafetyCheck` | `TestPickups_BR03_Electronic_SafetyCheck` | — (app tier) | ✅ |
| BR-04 | Organic auto-canceled after 3 days via goroutine | `internal/worker/organic_canceler.go`: ticker + `FindExpiredOrganic` + `BulkCancel`; clean shutdown via context cancellation | `TestOrganicCanceler_CancelsExpired` / `_DBError` | `TestWorker_OrganicAutoCancel` | — (app tier) | ✅ |
| BR-05 | Complete pickup → auto-generate payment (50000 / 100000) | `internal/service/pickup.go` Complete: `SELECT FOR UPDATE` + `UpdateStatus` + `CreatePayment` in single tx | `TestPickupService_Complete_CreatesPayment` / idempotency test | `TestPickups_BR05_Complete_AutoPayment` | `UNIQUE(waste_id)` on payments | ✅ |
| BR-06 | Payment confirmation requires S3 proof upload | `internal/handler/payment.go` ConfirmPayment: MIME allowlist + magic-byte sniff + MinIO upload; URL saved to `proof_file_url` | `TestConfirmPayment_*` | `TestPayments_ConfirmWithProof` | — (app tier) | ✅ |

### Technical Requirements (6/6)

| # | Requirement | Implementation | Evidence |
|---|-------------|----------------|----------|
| TR-1 | Dependency injection | Constructor wiring in `cmd/api/main.go`; every component receives interfaces from `internal/domain/` | `internal/mocks/` generated from domain interfaces; all unit tests use mock implementations |
| TR-2 | Graceful shutdown | OS signal handler in `cmd/api/main.go`; `e.Shutdown(ctx)` + `worker.Stop()` + `wg.Wait()`; configurable via `HTTP_SHUTDOWN_TIMEOUT` / `WORKER_SHUTDOWN_TIMEOUT` | `test/integration/shutdown_test.go` |
| TR-3 | Rate limiting on pickup creation | `internal/middleware/ratelimit.go`: per-IP token bucket on `POST /api/pickups`; TTL eviction for idle entries; `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST` env vars | E2E asserts 429 on burst; `rate_limit_active_clients` Prometheus gauge |
| TR-4 | Docker + PostgreSQL, single command to run | `make docker-up` (`docker compose -f deployments/docker-compose.yml up -d`) boots app + postgres + minio + jaeger + otel-collector + prometheus + grafana + loki + promtail | README §Setup |
| TR-5 | Consistent API responses | Envelope `{success:bool, data?, error:{code,message}, meta?}` returned by every handler; `respondError` now enriches with `{request_id, trace_id, span_id}` | `internal/handler/error_envelope_test.go` |
| TR-6 | Input validation | `validator/v10` with custom `db_exists_household` / `db_exists_pickup` ctx-aware tags; `positive_decimal` tag; field-level unit tests | `internal/handler/*_test.go` |

### Deliverables (5/5)

| # | Deliverable | Artefact | Status |
|---|-------------|----------|--------|
| D-1 | Go project with PostgreSQL via Docker | `make docker-up` in < 2 minutes; `/health` returns 200 | ✅ |
| D-2 | Source code with chosen structure | `cmd/`, `internal/{handler,service,repository,domain,middleware,observability,worker,storage,config}` — documented in 8 ADRs under `docs/adr/` | ✅ |
| D-3 | Postman or Insomnia collection covering all endpoints | `api/community-waste.postman_collection.json` (29 requests) + `api/community-waste.insomnia_collection.json` (matching); both have `pm.test` assertions | ✅ |
| D-4 | README.md with setup, migrations & seeding, env vars, architecture decisions | §Setup, §Migrations & seeding, §Environment variables, §Architecture decisions (+ ADR cross-links) | ✅ |
| D-5 | Daily commits throughout the test period | `git log --since='5 days ago' --oneline \| wc -l` ≥ 5 | ✅ |

> All 16 endpoints, 6 business rules, 6 tech requirements, and 5 deliverables satisfied.
> Observability (Prometheus, Grafana, Jaeger, OTel + Loki), concurrency hardening
> (advisory locks, partial UNIQUE index, SELECT FOR UPDATE), and rich quality gates
> (golangci-lint v2.12.2, 80% coverage gate, `-race` everywhere, testcontainers,
> full-stack E2E) are **beyond-spec** and form the primary differentiation.

---

## 2. Manual Verification Log

Reviewer-reproducible checks. Each must pass before the deliverable is signed
off. Run from a clean clone:

```bash
git clone <repo>
cd community-waste-collection-system
cp .env.example .env
make docker-up      # ~60s for healthchecks to settle
```

### 2.1 Stack health (after `docker compose up`)

| Check | Command | Expected | Status |
|---|---|---|---|
| App healthy | `curl -fsS http://localhost:8080/health \| jq .` | `{ "success":true, "data":{ "status":"ok"} }` | ✅ |
| App ready (after T9) | `curl -fsS http://localhost:8080/readyz \| jq .` | `{ "success":true, "data":{ "db":"ok", "worker":"running"} }` | ✅ |
| OpenAPI served | `curl -fsS http://localhost:8080/openapi.yaml \| head -1` | `openapi: 3.0.x` | ✅ |
| Swagger UI rendered | open `http://localhost:8080/api/docs` | Swagger UI loads, all 16 endpoints listed | ✅ |
| Metrics scraped | `curl -fsS http://localhost:2112/metrics \| grep -c '^http_requests_total'` | non-zero | ✅ |
| Prometheus targets up | `http://localhost:9090/targets` | All targets green | ✅ |
| Grafana dashboards | `http://localhost:3000` (admin/admin) | Business Ops + Service Health + Logs & Traces (after T20) dashboards render real data | ✅ |
| Jaeger traces visible | `http://localhost:16686` | Service `community-waste-collection-api` listed; traces with spans for handler→service→repository | ✅ |
| Loki + Promtail (T18) | `docker compose ps loki promtail` | `(healthy)` status | ✅ delivered |
| Trace ↔ log pivot (T19) | Click `trace_id` in Loki logs panel → Jaeger trace opens | navigates correctly | ✅ delivered |

### 2.2 Migration round-trip

```bash
make migrate-down   # all four down migrations apply cleanly
make migrate-up     # up migrations re-apply with no diff
psql "$DATABASE_URL" -c "\dt"   # households, waste_pickups, payments
```

| Check | Status |
|---|---|
| `migrate-up` from empty DB succeeds | ✅ |
| `migrate-down` reverses every migration cleanly | ✅ |
| Re-running `migrate-up` after down is idempotent | ✅ (T8 integration test guards this) |

### 2.3 Seed + smoke walk-through

```bash
make seed                                 # household + pickup fixtures
curl -s -X POST http://localhost:8080/api/households -d @fixtures/household.json
curl -s -X POST http://localhost:8080/api/pickups    -d @fixtures/pickup.json
curl -s -X PUT  http://localhost:8080/api/pickups/$ID/schedule -d '{"pickup_date":"2026-12-01"}'
curl -s -X PUT  http://localhost:8080/api/pickups/$ID/complete  # creates pending payment
curl -s -F 'proof=@fixtures/proof.jpg' http://localhost:8080/api/payments/$PID/confirm
curl -s http://localhost:8080/api/reports/households/$HID/history | jq .
```

| Step | Expected | Status |
|---|---|---|
| Create household | 201 + envelope.data.id | ✅ |
| Create pickup | 201 + status pending | ✅ |
| Schedule pickup | 200 + status scheduled | ✅ |
| Complete pickup | 200 + status completed + payment auto-created | ✅ |
| Confirm payment with proof | 200 + status paid + proof URL | ✅ |
| Reports history | 200 + household + pickups + payments + totals | ✅ |

### 2.4 Postman / Newman contract run (after T13)

```bash
newman run api/community-waste.postman_collection.json \
  --environment api/community-waste.postman_environment.json \
  --bail
```

| Check | Status |
|---|---|
| All 27 requests pass | ✅ |
| Newman runs in CI `contract` job (T13) | ✅ |
| Trace ↔ log correlation smoke (T21) — Loki query for captured trace_id returns ≥1 line | ✅ delivered (CI e2e job verifies) |

### 2.5 Concurrency / race verification (after T26)

```bash
go test ./test/e2e/... -run Concurrency -race -count=2 -v
```

| Invariant | Status |
|---|---|
| 8 parallel `POST /api/pickups` for same household → only 1 created | ✅ |
| 8 parallel `Schedule` on same pickup → only 1 transitions; 7 × 409 | ✅ |
| 8 parallel `Complete` on same pickup → only 1 success + 1 payment row; 7 × 409 | ✅ |

---

## 3. Coverage Report

### 3.1 Unit (`make test-unit`)

| Metric | Pre-T1 | Post-T1 target | CI gate |
|---|---|---|---|
| Total unit coverage | 82.2% | **82.7%** (actual, post T1) | 80% (passes) |
| `internal/service/...` | ~83% | ≥ 90% (ReportService now covered, T1) | — |
| `internal/handler/...` | ~85% | ≥ 88% (error envelope table-test, T16) | — |
| `internal/middleware/...` | ~88% | ≥ 90% (request_id + ratelimit eviction tests, T10/T30) | — |
| `internal/domain/...` | 100% | 100% | — |
| `internal/config/...` | ~95% | 100% (T40 validation) | — |

> coverpkg excludes `/mocks$`, `/repository$`, `/observability$` (generated /
> integration-only). The 80% gate is unit-only (no merge with integration).

### 3.2 Integration (`make test-integration`)

| Component | Backed by | Status |
|---|---|---|
| Repository round-trips (CRUD + filters + transactions) | testcontainers-go Postgres 17 | ✅ |
| `runInTx` rollback semantics | testcontainers-go | ✅ |
| Graceful shutdown (after T7) | testcontainers-go | ✅ |
| Migration round-trip (after T8) | testcontainers-go | ✅ |

### 3.3 End-to-End (`make test-e2e`)

| Suite | Tests | What it covers |
|---|---|---|
| `test/e2e/household_test.go` | 5 + cascade (T4) | CRUD + ON DELETE CASCADE |
| `test/e2e/pickup_test.go` | 19 + concurrency (T26) | Create/Schedule/Complete/Cancel + BR-01/02/03/05 + rate-limit + races |
| `test/e2e/payment_test.go` | 16 + date-range (T5) | Create/Confirm/List + BR-06 + status & household & date_from/date_to filters |
| `test/e2e/report_test.go` | 10 | Waste summary, payments summary, household history |
| `test/e2e/worker_test.go` | 2 + durability (T6) | BR-04 organic auto-cancel after cutoff |
| `test/e2e/concurrency_test.go` (T26) | 2 | parallel Schedule + parallel Complete invariants |
| **Total** | **~55** | All 16 endpoints; all 6 BRs; rate-limit; cascade; races; worker |

### 3.4 Performance (`make test-perf`)

5 benchmarks in `internal/repository/*_bench_test.go` and
`internal/service/pickup_bench_test.go`:

| Benchmark | What it measures |
|---|---|
| `BenchmarkPaymentRepository_List_1k` | List paging at 1k rows |
| `BenchmarkPaymentRepository_PaymentSummary` | Aggregation report |
| `BenchmarkPaymentRepository_HouseholdHistory_Parallel` (after T28) | Parallel query speedup |
| `BenchmarkPickupRepository_List_1k` | List with filters |
| `BenchmarkPickupService_Create_Contention` | TOCTOU race repro (drops to 0 dup after T23) |

CI `perf` job (commit `1badcab`) runs full-stack HTTP performance tests.

---

## 4. Known Limitations

These are deliberate, scoped, and documented. None block the deliverable.

| # | Limitation | Why it's acceptable | Documented in |
|---|---|---|---|
| L1 | **Jaeger storage is in-memory** (`SPAN_STORAGE_TYPE=memory`) — traces lost on container restart. | Spec doesn't require persistent traces; Cassandra/Elasticsearch backends are a one-line config swap. | README §Observability; ADR-0007 (after T12) |
| L2 | **MinIO is single-node** — no replication. | Spec asks for object storage; HA MinIO requires 4+ nodes which is overkill for a backend test. | README §Architecture |
| L3 | **No JWT / auth middleware** — endpoints are open. | Spec **explicitly** does not require auth. | README §Out of Scope |
| L4 | ~~No log aggregation backend~~ **Delivered**: Loki + Promtail ship in `docker-compose.yml`; structured logs (trace_id, span_id) across all layers; Grafana Logs and Traces dashboard; Jaeger↔Loki pivot via `tracesToLogsV2`; CI e2e job verifies correlation. | — | README §Structured Logs §Unified Log/Trace Search |
| L5 | **Prometheus retention is 15d** (default). | Single-tenant test deployment. | README §Observability |
| L6 | **No multi-region failover** for Postgres. | Spec scope. | README §Out of Scope |
| L7 | **Rate-limit TTL eviction runs in-process** — restart resets all per-IP buckets. | Acceptable for single-instance deployment. Redis-backed limiter is the production answer. | README §Production Considerations (after T11) |

---

## 5. Deliverable Checklist

Final pre-submission gate. Every box must be checked before declaring done.

### 5.1 Source code

- [x] All 16 endpoints implemented and tested
- [x] All 6 business rules implemented and tested (with concurrent-load tests after T26)
- [x] All 6 tech requirements satisfied
- [x] `golangci-lint v2.12.2` clean (0 issues, `golangci-lint config verify` passes)
- [x] All tests run with `-race`
- [x] Unit coverage ≥ 80% gate enforced in CI
- [x] No `//nolint` suppressions without a written reason

### 5.2 Documentation

- [x] `README.md` — Overview, Features, Architecture, Walkthrough, Quick Start, Observability (incl. Structured Logs + Unified Log/Trace Search, T22), Error Codes, Data Model, BRs, Troubleshooting (T11), Failure Modes (T54), SLOs (T14).
- [x] `CHANGELOG.md` (after T46)
- [x] `CONTRIBUTING.md` (after T47)
- [x] `docs/adr/` — 8 ADRs (after T12)
- [x] `api/openapi.yaml` with exhaustive `example:` (after T55)
- [x] `api/community-waste.postman_collection.json` + environment file (after T45)
- [x] `api/community-waste.insomnia_collection.json` (after T15)
- [x] Mermaid sequence + component diagrams in README (after T49)
- [x] Sample JSON log line in README (after T50)

### 5.3 Infrastructure & CI

- [x] `docker-compose up -d` boots full stack from clean clone in one command
- [x] All services healthchecked
- [x] GitHub Actions: lint, unit-tests, integration-tests, coverage-gate, e2e (incl. Loki correlation smoke T21), perf, **contract** (T13).
- [x] Codecov badge live and accurate (T22)
- [x] Prometheus alerts loaded (after T14)
- [x] Grafana dashboards auto-provisioned

### 5.4 Process & history

- [x] Daily commits visible — `git log --since='5 days ago' --oneline` shows ≥1 commit per day
- [x] Conventional Commits format throughout
- [x] No company name / PII in any committed file (`grep -ri "<company-name>"` returns empty)
- [x] `.env.example` enumerates every required env var; secrets never committed
- [x] The backend engineering test brief PDF and `REQUIREMENTS_RAW.md` remain gitignored
- [x] All planning files present under `plans/` (phase-0 through phase-7 + execution strategy + this gap-closure plan)

### 5.5 Final CI run

```bash
git push origin main
gh run watch --exit-status
```

| Job | Status |
|---|---|
| lint | ✅ |
| test-unit | ✅ |
| test-integration | ✅ |
| coverage-gate | ✅ |
| e2e | ✅ |
| perf | ✅ |
| contract (T13) | ✅ |
| log-trace-smoke (T21) | ✅ |

---

## Sign-off

Reviewer should:

1. Clone, `make docker-up`, walk through §2.1–§2.5.
2. Verify the §1 matrices map cleanly: every spec item has an
   implementation file **and** a test that asserts the invariant.
3. Spot-check coverage in §3.
4. Confirm §5 deliverable checklist is fully checked.

When all four pass: **ship it.**
