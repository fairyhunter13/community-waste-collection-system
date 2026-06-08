# Coverage Matrix

Maps every system feature to its production-code path, the tests that
exercise it, and the CI job that runs those tests on every push. This is
the living proof that each part of the system is implemented, tested, and
continuously verified.

---

## A. Entities

| Entity | Required fields | Domain file | Unit test |
|---|---|---|---|
| Household | id, owner_name, address, created_at, updated_at | `internal/domain/household.go:11-17` | `internal/handler/household_test.go::TestCreateHousehold_*` |
| Waste Pickup | id, household_id, type, status, pickup_date, safety_check, created_at, updated_at | `internal/domain/pickup.go:44-53` | `internal/service/pickup_test.go::TestSchedule_BR03_*` (safety_check), `TestComplete_BR05_Amount*` (type-driven amount) |
| Payment | id, household_id, waste_id, amount, payment_date, status, proof_file_url, created_at, updated_at | `internal/domain/payment.go:25-35` | `internal/service/payment_test.go::TestConfirm_*` (proof_file_url) |

### Enum Coverage

| Enum | Values | Definition | Test coverage |
|---|---|---|---|
| WasteType | organic, plastic, paper, electronic | `internal/domain/pickup.go:18-23` | `TestComplete_BR05_AmountOrganic`, `TestComplete_BR05_AmountPlastic`, `TestComplete_BR05_AmountPaper`, `TestComplete_BR05_AmountElectronic` |
| PickupStatus | pending, scheduled, completed, canceled | `internal/domain/pickup.go:26-31` | `test/e2e/pickup_test.go::TestPickup_FullLifecycle` (all 4 states) |
| PaymentStatus | pending, paid, failed | `internal/domain/payment.go:14-18` | paid path: `test/e2e/payment_test.go::TestPayment_Confirm_*`; failed path: `internal/service/payment_test.go::TestList_Filter_*` |

---

## B. Endpoints

| # | Method | Path | Handler (line) | Unit test | E2E test | CI job |
|---|---|---|---|---|---|---|
| 1 | POST | /api/households | `handler.go:169` → `CreateHousehold` | `household_test.go::TestCreateHousehold_*` | `e2e/household_test.go::TestHousehold_CRUD` | `test-unit`, `e2e` |
| 2 | GET | /api/households | `handler.go:170` → `ListHouseholds` | `household_test.go::TestListHouseholds_*` | `TestHousehold_Pagination` | `test-unit`, `e2e` |
| 3 | GET | /api/households/:id | `handler.go:171` → `GetHousehold` | `household_test.go::TestGetHousehold_*` | `TestHousehold_CRUD` | `test-unit`, `e2e` |
| 4 | DELETE | /api/households/:id | `handler.go:172` → `DeleteHousehold` | `household_test.go::TestDeleteHousehold_*` | `TestHousehold_DeleteCascades_PickupsAndPayments` | `test-unit`, `e2e` |
| 5 | POST | /api/pickups | `handler.go:175` → `CreatePickup` + rate limiter | `pickup_test.go::TestCreatePickup_*` | `TestPickup_FullLifecycle`, `TestPickup_BR01_*` | `test-unit`, `e2e` |
| 6 | GET | /api/pickups | `handler.go:176` → `ListPickups` | `pickup_test.go::TestListPickups_*` | `TestPickup_FilterByStatus`, `TestPickup_FilterByHouseholdID` | `test-unit`, `e2e` |
| 7 | PUT | /api/pickups/:id/schedule | `handler.go:177` → `SchedulePickup` | `pickup_test.go::TestSchedulePickup_*` | `TestPickup_FullLifecycle`, `TestPickup_BR03_*` | `test-unit`, `e2e` |
| 8 | PUT | /api/pickups/:id/complete | `handler.go:178` → `CompletePickup` | `pickup_test.go::TestCompletePickup_*` | `TestPickup_FullLifecycle`, `concurrency_test.go::TestConcurrent_Complete_OnlyOneSucceeds` | `test-unit`, `e2e` |
| 9 | PUT | /api/pickups/:id/cancel | `handler.go:179` → `CancelPickup` | `pickup_test.go::TestCancelPickup_*` | `TestPickup_Cancel`, `TestPickup_CancelCompleted` | `test-unit`, `e2e` |
| 10 | POST | /api/payments | `handler.go:181` → `CreatePayment` | `payment_test.go::TestCreatePayment_*` | `TestPayment_*`, `TestPayment_CrossHouseholdRejected` | `test-unit`, `e2e` |
| 11 | GET | /api/payments | `handler.go:182` → `ListPayments` | `payment_test.go::TestListPayments_*` | `TestPayment_*` (filter coverage) | `test-unit`, `e2e` |
| 12 | PUT | /api/payments/:id/confirm | `handler.go:183` → `ConfirmPayment` | `payment_test.go::TestConfirmPayment_*` | `TestPayment_Confirm_*`, `TestConfirmPayment_RejectsImageJpg` | `test-unit`, `e2e` |
| 13 | GET | /api/reports/waste-summary | `handler.go:186` → `WasteSummary` | `report_test.go::TestWasteSummary_*` | `e2e/report_test.go::TestWasteSummary_*` | `test-unit`, `e2e` |
| 14 | GET | /api/reports/payment-summary | `handler.go:187` → `PaymentSummary` | `report_test.go::TestPaymentSummary_*` | `e2e/report_test.go::TestPaymentSummary_*` | `test-unit`, `e2e` |
| 15 | GET | /api/reports/households/:id/history | `handler.go:188` → `HouseholdHistory` | `report_test.go::TestHouseholdHistory_*` | `TestHouseholdHistory_200`, `TestHouseholdHistory_WithPickupsAndPayments` | `test-unit`, `e2e` |

Extra endpoints (not in the 15): `/health` (liveness), `/readyz` (readiness + DB ping).

---

## C. Business Rules

| BR | Rule | Enforcement | DB safety net | Unit test | E2E test | CI job |
|---|---|---|---|---|---|---|
| BR-01 | Pending payment blocks new pickup | `service/pickup.go:Create` — `HasPendingPaymentForHousehold` + `pg_advisory_xact_lock` | Partial UNIQUE index `uq_pickups_pending_per_household` | `pickup_test.go::TestCreate_BR01_*` | `e2e/pickup_test.go::TestPickup_BR01_*`, `concurrency_test.go::TestPickups_ConcurrentCreate_SameHousehold_AdvisoryLockSerializes` | `test-unit`, `e2e` |
| BR-02 | Only pending → scheduled | `service/pickup.go:113` conditional `UPDATE WHERE status='pending'` → `ErrConflict` on 0 rows | — | `pickup_test.go::TestSchedule_BR02_*` | `TestPickup_BR02_ScheduleCompleted_Fails`, `concurrency_test.go::TestConcurrent_Schedule_OnlyOneSucceeds` | `test-unit`, `e2e` |
| BR-03 | Electronic requires safety_check | `service/pickup.go:141` | — | `pickup_test.go::TestSchedule_BR03_*` | `TestPickup_BR03_ElectronicRequiresSafetyCheck`, `TestPickup_BR03_ElectronicWithSafetyCheck_CanSchedule` | `test-unit`, `e2e` |
| BR-04 | Organic auto-cancel after N days (goroutine, clean shutdown) | `worker/organic_canceler.go:30-120` (ctx-cancel-aware) | — | `worker/organic_canceler_test.go::Test*` | `e2e/worker_test.go::TestWorker_*`, `e2e/shutdown_test.go::TestShutdown_*` | `test-unit`, `e2e` |
| BR-05 | Complete → auto-payment (50K organic/plastic/paper, 100K electronic) | `service/pickup.go:182-264` — tx: `UpdateStatus + CreateWithTx + Commit` | Partial UNIQUE `uq_payments_one_pending_per_household` | `pickup_test.go::TestComplete_BR05_Amount{Organic,Electronic,Plastic,Paper}` | `concurrency_test.go::TestConcurrent_Complete_OnlyOneSucceeds`, `TestPayments_ConcurrentDirectCreate_PartialUniqueWins` | `test-unit`, `e2e` |
| BR-06 | Confirm requires proof file upload to S3 | `handler/payment.go:104-163` (MIME allowlist + magic-byte sniff); `service/payment.go:Confirm` (MinIO PUT + DB UPDATE) | — | `payment_test.go::TestConfirm_*` | `TestPayment_Confirm_Success`, `TestConfirmPayment_RejectsImageJpg` | `test-unit`, `e2e` |

---

## D. Technical Requirements

| TR | Requirement | Wiring | Verification |
|---|---|---|---|
| TR-1 | Dependency injection | Constructor injection: `service.NewPickupService(repo, paymentRepo, db)`; wired in `cmd/api/main.go:30-55` | All `*_test.go` files inject mocks via `mocks.NewPickupRepository(t)` — proves DI seams exist; verified via `internal/service/pickup_test.go::TestCreate_*` |
| TR-2 | Graceful shutdown | `cmd/api/main.go:160-210` — `signal.Notify(SIGINT, SIGTERM)` → cancel worker ctx → `wg.Wait` → `e.Shutdown` → metrics server shutdown → tracer flush | `test/e2e/shutdown_test.go::TestShutdown_*` (sends SIGTERM via Docker, asserts in-flight requests complete) |
| TR-3 | Rate limit on pickup creation only | `handler/handler.go:175` chains `middleware.RateLimiter(cfg)` only on `POST /api/pickups` | `internal/middleware/middleware_test.go::TestRateLimit_*`, `test/e2e/phase10_test.go::TestRateLimit_429EnvelopeWithMeta` |
| TR-4 | Docker single-command stack | `deployments/docker-compose.yml` — app + postgres + minio + jaeger + loki + promtail + grafana; `docker compose up --build -d` | E2E + Perf CI jobs both spin the stack via `docker compose up --build` |
| TR-5 | Consistent envelope + status codes | `respond`/`respondError`/`respondList` at `handler.go:230-310`; `echoErrorHandler` at `handler.go:99` maps all error types to canonical codes | `internal/handler/error_envelope_test.go::Test*`, `test/e2e/error_visibility_test.go::TestErrorVisibility` |
| TR-6 | Input validation | `validator.v10` + custom DB-backed validators (`db_exists_household`, `db_exists_pickup`, `positive_decimal`) registered at `handler.go:26-90` | `internal/handler/*_test.go::Test*_400_*` (every handler has a 400 case) + `test/e2e/*_test.go::Test*_400_*` |

---

## E. Deliverables

| # | Deliverable | Location | Verified by |
|---|---|---|---|
| 1 | Go project + Postgres via Docker (single command) | `Makefile`, `deployments/docker-compose.yml` — `make up` starts the full stack | E2E + Perf CI jobs (`ci.yml`) |
| 2 | Source with chosen structure (layered / hexagonal) | `cmd/`, `internal/{domain,handler,service,repository,middleware,observability,storage,worker,config}` | Architecture described in `docs/architecture.md` |
| 3 | Postman + Insomnia API collections | `api/community-waste.postman_collection.json` (27 requests), `api/community-waste.insomnia_collection.json` (27 requests) | `contract` CI job — Python parity check + Newman smoke test |
| 4 | README (setup, migrations, env vars, architecture decisions) | `README.md` — §Prerequisites, §Quick Start, §Environment Variables, §Architecture Decisions | CI lint; regression-checked in deployment notes |
| 5 | Versioned repo with regular commits | Git history — daily commits during development window | `git log --oneline` |

---

## F. CI Integration

| Job (`ci.yml`) | What it runs | Covers |
|---|---|---|
| `lint` | golangci-lint v2.12.2 | Code quality; gates all other jobs |
| `test-unit` | `go test -race -coverprofile ./internal/... -v` | All handler/service/middleware/worker unit tests |
| `test-integration` | `go test -tags=integration ./internal/repository/... ./internal/service/...` | Repository + service integration via testcontainers |
| `coverage-gate` | Enforces `> 80 %` strict; uploads to Codecov | Guards coverage regression |
| `e2e` | Docker Compose stack + `go test -tags=e2e ./test/e2e/...` + Newman smoke | Full request flows, all BRs, TR-2 graceful shutdown |
| `perf` | Docker Compose stack + `go test -bench -tags=perf ./test/perf/...` | Latency + throughput baselines |
| `vuln` | govulncheck | Dependency CVE scan |
| `image-scan` | Trivy HIGH/CRITICAL | Container CVE scan |
| `contract` | OpenAPI Redocly lint + Postman/Insomnia parity + route-extract | Spec ↔ code alignment |

Auxiliary workflows (`.github/workflows/`):
- `dashboards.yml` — Grafana dashboard lint + metric-existence check + Playwright UI (path-triggered on `test/dashboards/**`).
- `e2e.yml` — weekly cron + manual dispatch; same E2E suite.
- `load.yml` — manual k6 dispatch.

**Net effect:** Every new test added under `internal/` is automatically
picked up by `test-unit`, `test-integration`, and `coverage-gate`.
Every new test under `test/e2e/` is picked up by `e2e`. No workflow
changes are needed when adding tests.
