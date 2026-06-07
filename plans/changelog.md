# Product history

## v3 — Scope cleanup

Removed over-engineering that exceeded the product spec:
- OTel collector intermediary replaced with direct Jaeger OTLP export
- `/api/version`, `/api/docs`, and Swagger UI removed
- Repo policy surface removed (CODEOWNERS, dependabot, PR/issue templates,
  CHANGELOG, CONTRIBUTING, SECURITY, .editorconfig)
- ADRs collapsed into `README.md ## Architecture decisions`
- Binary release tooling removed (goreleaser, release workflow)
- SBOM CI job removed
- Hot-reload dev loop removed (air, .air.toml)
- Grafana dashboard correctness test suite added (`test/dashboards/`)
- Surgical unit / integration / E2E / perf / load test additions to close gaps

## v2 — Observability + hardening

Added full observability stack: Prometheus RED metrics, slog JSON logs,
distributed tracing (OTel → Jaeger), Grafana dashboards (auto-provisioned),
Loki/Promtail log aggregation, log/trace correlation via `trace_id`.

Hardened API surface: custom `HTTPErrorHandler` with consistent error
envelope on all error paths including framework-level errors; body limit
middleware; X-Request-ID injection; strict pagination validation; per-IP
rate-limiter memory TTL cleanup.

Added supply-chain CI: govulncheck, Trivy container scan, coverage gate
(≥80%), Codecov upload, OpenAPI spec lint (Redocly), API collection
validation, E2E and load-test workflows.

## v1 — Core API

Implemented all 16 REST endpoints across households, pickups, payments, and
reports. Enforced all six business rules (BR-01..BR-06) in the service layer.
Added background worker for BR-04 organic-pickup auto-cancellation.
Docker Compose single-command boot with PostgreSQL and MinIO.
Unit tests, integration tests (testcontainers), and E2E tests.
Postman and Insomnia collections.
