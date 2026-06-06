# ADR 0003 — `shopspring/decimal` for monetary amounts

**Status:** Accepted

## Context

The system charges per pickup and tracks payment revenue in summaries.
`float64` is unacceptable for money: 0.1 + 0.2 ≠ 0.3, and rounding
behaviour varies across hardware. The wire format also needs to be a
predictable, locale-independent string.

## Decision

Store monetary amounts as `NUMERIC(12,2)` in PostgreSQL and use
`github.com/shopspring/decimal` (`decimal.Decimal`) throughout the Go
codebase. The type implements `database/sql.Scanner` natively and
marshals to JSON as a quoted string (e.g. `"50000.00"`), matching the
contract documented in the API reference.

## Consequences

- Arithmetic is exact and round-trips through SQL without loss.
- JSON responses use a stable string form; clients never have to
  reason about float precision.
- Comparisons require `Decimal.Equal` / `Decimal.Cmp` rather than `==`.
  We accept that — it's a one-line idiom and the alternative is silent
  wrongness.
