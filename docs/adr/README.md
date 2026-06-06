# Architecture Decision Records

This directory contains the architecture decisions made for the
Community Waste Collection API. Each ADR follows the lightweight
[MADR-style](https://adr.github.io/madr/) format: a single short
paragraph capturing the decision, its context, and its consequences.

## Index

| # | Decision | Status |
|---|----------|--------|
| [0001](0001-no-orm.md) | No ORM — raw SQL via `sqlx` | Accepted |
| [0002](0002-sentinel-errors.md) | Sentinel errors for domain outcomes | Accepted |
| [0003](0003-shopspring-decimal.md) | `shopspring/decimal` for monetary amounts | Accepted |
| [0004](0004-per-ip-rate-limit.md) | Per-IP token bucket rate limiting | Accepted |
| [0005](0005-worker-context-cancellation.md) | Background worker with context cancellation | Accepted |
| [0006](0006-business-rules-in-service-layer.md) | Business rules enforced in the service layer | Accepted |
| [0007](0007-opentelemetry.md) | OpenTelemetry — vendor-neutral distributed tracing | Accepted |
| [0008](0008-prometheus-red-metrics.md) | Prometheus + Grafana — RED metrics with auto-provisioning | Accepted |

## When to write a new ADR

Add a new file when introducing a decision that:

- changes how the service stores, serialises, or validates data;
- introduces a new cross-cutting concern (auth, observability, rate
  limiting, scheduling);
- swaps out a previously chosen technology;
- adopts or abandons a third-party library that materially shapes
  the codebase.

Number sequentially; never re-number existing ADRs. To supersede an
ADR, add a new one and mark the old one **Superseded by NNNN**.
