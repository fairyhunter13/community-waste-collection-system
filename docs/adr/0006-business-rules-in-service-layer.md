# ADR 0006 — Business rules enforced in the service layer

**Status:** Accepted

## Context

The spec defines six business rules (BR-01..BR-06) that constrain
pickup and payment lifecycles. These rules cross boundaries: a single
rule may touch the pickups table, the payments table, and trigger
downstream side effects (e.g. auto-creating a payment on completion).

## Decision

All business-rule enforcement lives in `internal/service/*`. Handlers
parse and validate input only; repositories are pure data access. The
service layer:

- composes transactions across repositories;
- holds advisory locks and `SELECT ... FOR UPDATE` discipline where
  TOCTOU windows matter;
- maps repository outcomes to domain sentinels.

## Consequences

- Each business rule has exactly one enforcement site, by design.
- Unit tests cover business rules with mocked repositories; integration
  tests prove the SQL invariants hold; E2E tests prove the
  enforcement is reachable through the HTTP surface.
- Adding a new endpoint that touches an existing entity must go through
  the existing service method — bypassing it would require an obvious,
  reviewable change.
