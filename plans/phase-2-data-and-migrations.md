# Phase 2 — Data and Migrations

Covers the database schema, migration strategy, and repository layer.

## Schema

Three core tables managed by five golang-migrate SQL files under `migrations/`:

**households** — owner name and address; referenced by waste_pickups and payments.

**waste_pickups** — one row per pickup request. Key columns:
- `type` TEXT: `organic` | `plastic` | `paper` | `electronic` | `hazardous`
- `status` TEXT: `pending` → `scheduled` → `completed` | `canceled`
- `pickup_date` TIMESTAMPTZ nullable (set when scheduled)
- `safety_check` BOOLEAN (required `true` for electronic type before scheduling)

**payments** — one row per completed pickup. Key columns:
- `waste_id` UUID references `waste_pickups(id)`
- `amount` NUMERIC(12,2) — server-derived from `domain.PaymentAmounts`
- `status` TEXT: `pending` | `paid`
- `proof_file_url` TEXT nullable (set on confirm)
- `payment_date` TIMESTAMPTZ nullable (set on confirm)

## Migrations

| File | Purpose |
|------|---------|
| `000001_init.up.sql` | Create `households`, `waste_pickups`, `payments` tables |
| `000002_create_pickups.up.sql` | Add lookup indexes on `waste_pickups(household_id)`, `(status)`, `(type, status)`, `(created_at)` — all `IF NOT EXISTS` |
| `000003_create_payments.up.sql` | Add lookup indexes on `payments(household_id)`, `(waste_id)`, `(status, created_at)` — all `IF NOT EXISTS` |
| `000004_add_unique_pending_payment.up.sql` | Partial UNIQUE index `uq_payments_one_pending_per_household` on `payments(household_id) WHERE status='pending'` — DB-level BR-01 guard |
| `000005_add_performance_indexes.up.sql` | Composite indexes for list+filter queries |

Migrations run automatically at startup via `golang-migrate`. `make migrate-up` and
`make migrate-down` drive them manually.

## Repository layer

Repository implementations live in `internal/repository/`. Each struct embeds a
`*sqlx.DB` and accepts an optional `sqlx.ExtContext` parameter on write methods
so the service layer can pass a transaction handle (`*sqlx.Tx`) for atomic operations.

Key patterns:

- **Conditional UPDATE status guards** — `UpdateStatus` builds a `WHERE id=? AND status=?`
  query based on the target status, returning `ErrNotFound` (0 rows affected) or
  `ErrConflict` (wrong current status). See `internal/repository/pickup.go:UpdateStatus`.
- **Partial-UNIQUE conflict** — `CREATE`/`CreateWithTx` on `payments` returns a wrapped
  `ErrConflict` when the partial UNIQUE index fires (duplicate pending payment for a household).
- **`IF NOT EXISTS` on all indexes** — prevents migration re-run errors on fresh databases
  that already have the indexes from a prior partial run.

## Payment amounts (server-side derivation)

`domain.PaymentAmounts` in `internal/domain/payment.go` maps each waste type to its
canonical NUMERIC amount. The service layer uses this map when creating a payment — the
client-supplied `amount` field is silently ignored to prevent price manipulation.

## Verification

- `make migrate-up && make migrate-down` — exercises all five migration pairs.
- `go test ./internal/repository/...` — repository unit tests use testcontainers
  (PostgreSQL) to run against a real DB.
- `make test-integration` — full service+repository integration tests.
