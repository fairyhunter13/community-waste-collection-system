# Engineering Plans

This directory holds product-engineering records for the Community Waste
Collection API.

## Index

| File | Contents |
|------|----------|
| [architecture.md](architecture.md) | Cross-cutting reference: layer responsibilities, business rule enforcement, DB schema, observability, graceful shutdown |
| [phase-1-foundations.md](phase-1-foundations.md) | DI wiring, Echo setup, config, graceful shutdown, sentinel errors |
| [phase-2-data-and-migrations.md](phase-2-data-and-migrations.md) | Database schema, migrations, repository layer, sqlx patterns |
| [phase-3-business-rules.md](phase-3-business-rules.md) | BR-01..BR-06 service-layer enforcement |
| [phase-4-handlers-and-validation.md](phase-4-handlers-and-validation.md) | HTTP handlers, input validation, MIME allowlist, rate limiting |
| [phase-5-observability.md](phase-5-observability.md) | Metrics, structured logging, distributed tracing, Grafana dashboards |
| [phase-6-testing.md](phase-6-testing.md) | Unit, integration, E2E, concurrency, load tests |
| [phase-7-delivery-polish.md](phase-7-delivery-polish.md) | Docker Compose, Makefile targets, CI workflow, OpenAPI contract |
