# Phase 3 — Business Rules

All six business-rule invariants are enforced exclusively in `internal/service/`.
Handlers do input parsing and validation only; repositories are pure data access.

## BR-01 — Single open billing cycle per household

**Location**: `internal/service/pickup.go:Create` (line ~48)

A household cannot create a new pickup while it has a `pending` payment.
Enforcement uses two complementary guards:

1. **Application-layer check**: `HasPendingPaymentForHousehold` queries
   `EXISTS(SELECT 1 FROM payments WHERE household_id=$1 AND status='pending')`.
   Returns `ErrConflict` (→ HTTP 409) if the household has a pending payment.
2. **Database-layer guard**: Partial UNIQUE index `uq_payments_one_pending_per_household`
   on `payments(household_id) WHERE status='pending'` (migration 000004).
   Catches concurrent inserts that slip past the application check.

## BR-02 — Status state machine

**Location**: `internal/service/pickup.go` (Schedule, Complete, Cancel)

Valid transitions:
- `pending` → `scheduled` (Schedule)
- `scheduled` → `completed` (Complete)
- `pending` | `scheduled` → `canceled` (Cancel)

Each transition uses a conditional `UPDATE … WHERE id=? AND status=<expected>`.
Zero rows affected means wrong status → `ErrConflict` (→ HTTP 409).

## BR-03 — Electronic pickup safety check

**Location**: `internal/service/pickup.go:Schedule` (line ~130)

Scheduling an `electronic` pickup requires `safety_check: true` on the pickup record.
If `safety_check` is false, returns `ErrBusinessRule` (→ HTTP 422).

## BR-04 — Organic auto-cancellation

**Location**: `internal/worker/organic_canceler.go`

The `OrganicCanceler` worker ticks on `WORKER_CANCEL_INTERVAL` (default: 1h).
Each tick:
1. Calls `FindExpiredOrganic` — `SELECT id FROM waste_pickups WHERE type='organic' AND status='pending' AND created_at < now() - INTERVAL '$N days'` with a configurable query timeout.
2. Calls `BulkCancel` — `UPDATE waste_pickups SET status='canceled' WHERE id=ANY($1) AND status IN ('pending','scheduled')` (status guard prevents overwriting completed pickups).

Worker exits cleanly on context cancellation (SIGTERM drain).

## BR-05 — Atomic pickup completion with payment creation

**Location**: `internal/service/pickup.go:Complete` (line ~183)

Completing a `scheduled` pickup atomically creates a `pending` payment record.
Implementation:

1. `FindByID` — load pickup, verify `status == scheduled` (else `ErrConflict`).
2. Derive canonical payment amount from `domain.PaymentAmounts[pickup.Type]`.
3. `db.BeginTxx(ctx, nil)` — start transaction.
4. `UpdateStatus(ctx, id, completed, tx)` — conditional UPDATE WHERE status='scheduled'.
5. `CreateWithTx(ctx, tx, payment)` — INSERT payment.
6. `tx.Commit()`.

If any step fails, deferred `tx.Rollback()` undoes partial changes.

## BR-06 — Payment confirmation requires proof file

**Location**: `internal/handler/payment.go:ConfirmPayment` + `internal/service/payment.go:Confirm`

Two-layer content validation:

1. **Handler (MIME allowlist)**: inspects the multipart `Content-Type` header — must be
   `image/jpeg`, `image/png`, or `application/pdf`.
2. **Handler (magic-byte sniff)**: reads the first 512 bytes via `http.DetectContentType`;
   rejects if the sniffed type is not in the allowlist. Reader is rewound via
   `io.MultiReader` so the full file body reaches the service.
3. **Service**: `nil` reader check → `ErrValidation`; already-paid check → `ErrConflict`;
   uploads to MinIO via `StorageService.Upload`; writes `proof_file_url` + `payment_date`
   to DB. If DB write fails after upload, best-effort `StorageService.Delete` prevents
   S3 orphan objects.

## Verification

- `go test ./internal/service/...` — unit tests for all six rules.
- `go test ./internal/handler/...` — handler tests for BR-06 MIME and magic-byte rejection.
- `make test-e2e` — E2E tests confirm the full HTTP ↔ DB flow for each rule.
- `test/e2e/concurrency_test.go` — concurrent BR-01 and BR-05 tests confirm DB guards hold.
