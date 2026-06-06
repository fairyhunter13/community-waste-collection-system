# ADR 0001 — No ORM, raw SQL via `sqlx`

**Status:** Accepted

## Context

The data model is small (4 tables) and the access patterns are
well-bounded by the spec (16 endpoints, several reports). An ORM would
add a layer of abstraction whose query-generation behaviour must itself
be audited for the BR-01..BR-06 invariants.

## Decision

Use raw SQL strings executed via `github.com/jmoiron/sqlx`. `sqlx`
provides struct scanning and named-parameter binding without
introducing query-builder semantics or migration tooling — those stay
explicit (`golang-migrate`) and reviewable.

## Consequences

- Every query is visible at the call site and trivially `EXPLAIN`-able.
- Performance tuning (e.g. composite indexes for BR-01 hot paths) does
  not have to fight an abstraction layer.
- The cost is repetition: each repository method writes out its own
  SELECT / INSERT / UPDATE. We accept that — the codebase is small
  enough that the duplication is reviewable.
