# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added — Phase 6 (adversarial gap closure)

- **Tier 1 — test depth**: report service unit test, pickup `Complete`
  idempotency + transaction rollback tests, worker durability test, cascade
  delete E2E, payment date-range filter E2E.
- **Tier 2 — production polish**: `/readyz` separate from `/health`,
  `X-Request-ID` middleware (precedence: inbound → OTel trace_id → fresh UUID),
  README troubleshooting + prerequisites blocks, `docs/adr/` directory with
  eight architecture decision records.
- **Tier 3 — beyond-spec hardening**: CI `contract` job validating Postman /
  Insomnia / OpenAPI specs, Prometheus alert rules (5xx rate, p99 latency,
  worker stalled, API down), Insomnia v4 export, error envelope shape test.
- **Tier 5 — concurrency / data integrity**: closed three TOCTOU races
  (BR-01 advisory lock, BR-02 conditional UPDATEs, BR-05 `SELECT … FOR UPDATE`
  inside the Complete tx), concurrent-Complete E2E, partial UNIQUE index
  `uq_payments_one_pending_per_household` at the schema tier.
- **Tier 6 — perf + hardening**: `errgroup` parallelisation of
  `HouseholdHistory` (single-RTT), composite indexes on hot query paths,
  rate-limiter TTL eviction with active-clients gauge, worker panic recovery
  with `WorkerCyclesFailedTotal`, explicit HTTP timeouts, per-route
  `BodyLimit`, MIME allowlist + sanitised error mapping on /confirm,
  `Secure()` middleware, `SetConnMaxLifetime` + `application_name` DSN
  injection + DB pool gauges, `Config.Validate()`, split
  HTTP / worker shutdown timeouts, Dockerfile nonroot + `HEALTHCHECK`.

### Added — Phase 5 (post-delivery fixes)

- BR-04 worker documented in README; corrected BR-01 / BR-05 / BR-06 wording.
- E2E pagination total-count fix; non-nil JSON lists; CI infra port-conflict
  resolution; OTel → Jaeger trace pipeline.
- Codecov gate (≥80%), CI `coverage-gate` step, infra packages excluded
  from coverage report.

### Added — Phases 0–4 (initial delivery)

- 16 endpoints: 4 households + 5 pickups + 3 payments + 3 reports + /health
  + OpenAPI/Swagger.
- All six business rules enforced in the service layer.
- DI constructor wiring, graceful shutdown, per-IP rate limiter on
  `POST /api/pickups`, single-command Docker Compose stack with seven services
  and health checks, consistent envelope responses, validator/v10 with custom
  `db_exists_*` rules.
- Observability: 14 Prometheus instruments, 2 Grafana dashboards, OTel →
  Jaeger pipeline, slog JSON logging with trace-id correlation.
- Quality gates: golangci-lint v2.12.2 (0 issues), 80% unit-coverage gate,
  `-race` on every test invocation, testcontainers-backed integration + full
  E2E (36+ tests covering BR-01..BR-06).
