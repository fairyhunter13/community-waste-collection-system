# ADR 0002 — Sentinel errors for domain outcomes

**Status:** Accepted

## Context

The handler layer must map service-layer outcomes to HTTP status codes
without leaking SQL- or repository-specific error types into the wire
contract. We needed a stable, typed way for services to signal
"not found", "conflict", "business-rule violation", and "validation
failed" independent of the transport.

## Decision

Define five sentinel errors in `internal/domain`:

- `ErrNotFound` → 404
- `ErrConflict` → 409
- `ErrBusinessRule` → 422
- `ErrValidation` → 400
- `ErrInternalFailure` → 500

Services wrap their underlying errors with these sentinels via
`fmt.Errorf("...: %w", domain.ErrXxx)`. Handlers check with
`errors.Is(err, domain.ErrXxx)`.

## Consequences

- HTTP status mapping lives in one place (`handler.mapError`).
- Repositories can return any error; services translate to domain
  outcomes once.
- New outcome categories require adding a sentinel and a `mapError`
  branch — both are obvious during review.
