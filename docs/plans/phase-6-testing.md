# Phase 6 — Testing

Four testing layers provide complementary coverage: unit, integration, end-to-end,
and load.

## Unit tests

Location: `*_test.go` files alongside production code, package `xxx_test`.

Frameworks: `testify/suite` for table suites, `testify/mock` + mockery-generated
mocks in `internal/mocks/` for interface substitution.

Key suites:

| Suite | File | What it covers |
|-------|------|----------------|
| `HouseholdHandlerSuite` | `internal/handler/household_test.go` | 201/400/404/204 for all household endpoints |
| `PickupHandlerSuite` | `internal/handler/pickup_test.go` | All 6 pickup endpoints including state-machine rejections |
| `PaymentHandlerSuite` | `internal/handler/payment_test.go` | MIME allowlist, magic-byte sniff, 200/400/404 |
| `HouseholdServiceSuite` | `internal/service/household_test.go` | Create/GetByID/List/Delete including ErrNotFound propagation |
| `PickupServiceSuite` | `internal/service/pickup_test.go` | BR-01..BR-04 enforcement, Complete atomicity |
| `PaymentServiceSuite` | `internal/service/payment_test.go` | BR-05 canonical amount derivation, BR-06 nil-file guard, S3 error propagation |
| `ReportServiceSuite` | `internal/service/report_test.go` | HouseholdReport, WasteSummary aggregation |
| `RatelimitSuite` | `internal/middleware/ratelimit_test.go` | Per-IP limiting, eviction |
| `S3ClientTests` | `internal/storage/s3_test.go` | Upload success/failure, Ping |
| `HealthSuite` | `internal/handler/health_test.go` | `/health` 200, `/readyz` DB-down 503, storage-down 503 |

Run: `go test ./...` or `make test`.

## Integration tests

Location: `internal/*/integration_test.go`, build tag `integration`.

Use `testcontainers-go` to spin up a real PostgreSQL container per suite.
Each repository implementation is exercised against the actual DB schema,
including migration application. The partial-UNIQUE index (BR-01) and
conditional-UPDATE guards (BR-02/05) are tested against concurrent writers.

Run: `make test-integration`.

## End-to-end tests

Location: `test/e2e/`, build tag `e2e`.

Spin up the full `docker compose` stack (PostgreSQL + MinIO + the API binary)
and drive it with HTTP requests. One file per domain area:

| File | Scenarios |
|------|-----------|
| `household_test.go` | CRUD lifecycle |
| `pickup_test.go` | Full status machine, BR-02/03/04 |
| `payment_test.go` | BR-06 proof upload, canonical amount, MIME-lie rejection |
| `concurrency_test.go` | E4: concurrent BR-01 (only one pickup created), E5: concurrent BR-05 |
| `report_test.go` | Household report, waste summary totals |

Run: `make test-e2e`.

## Concurrency tests (data integrity)

`test/e2e/concurrency_test.go` fires concurrent goroutines at the same household
to verify that partial-UNIQUE and conditional-UPDATE DB guards prevent double-billing
and double-completion even under race conditions.

## Load tests

Location: `test/load/`, run via k6.

`k6 run test/load/smoke.js` — smoke test (low VU, short duration).
`k6 run test/load/stress.js` — ramp to high concurrency, measure p95 latency.

Run: `make load-test` (requires k6 installed or `make load-test-docker`).

## Coverage

Codecov gate at 80% line coverage, enforced in CI via `make coverage`.

## Verification

```bash
make test                # unit tests
make test-integration    # integration tests (requires Docker)
make test-e2e            # end-to-end tests (requires Docker)
go test -race ./...      # race detector (all packages)
make coverage            # HTML coverage report + 80% gate check
```
