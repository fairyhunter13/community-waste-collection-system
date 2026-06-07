# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added — Phase 11 (E2E proofs + spec drift + DX polish)

- **Tier 14 — E2E proof for Phase-10 fixes**: six new E2E tests in
  `test/e2e/phase10_test.go` proving every Phase-10 fix holds end-to-end:
  cross-household payment rejection (422), pagination 400 on invalid input,
  rate-limit 429 envelope with trace meta, inbound W3C `traceparent`
  propagated into response `trace_id`, body-limit 413 enveloped response,
  and `image/jpg` rejection from the MIME allowlist.
- **`echoErrorHandler`**: custom Echo error handler normalises all
  framework-level errors (413, 429, 404, 405, 500) into the same envelope
  format as application errors, including `meta.{trace_id, span_id,
  request_id}` when an OTel span is active.
- **Tier 15 — OpenAPI spec drift**: added `ErrorMeta` schema documenting
  `request_id`, `trace_id`, `span_id`; updated `ErrorResponse` to include
  optional `meta`; added exhaustive `example:` blocks on all 14 API
  operations (request bodies, 2xx responses, all 4xx/5xx envelopes);
  fixed `WasteSummaryResponse` and `PaymentSummaryResponse` examples to
  match actual schemas; referenced `RequestTooLarge` on POST operations;
  added `security: []` at root level; lint now passes with 0 errors.
- **Tier 16 — docs / DX**: `CHANGELOG.md` Phase 9 + Phase 10 + Phase 11
  sections; `plans/phase-10-adversarial-closeout.md` plan record; `.air.toml`
  hot-reload config + `make dev` target + CONTRIBUTING.md paragraph.
- **Tier 17 — Syft SBOM**: CI `sbom` job generates a CycloneDX SBOM of the
  source tree and uploads it as a workflow artifact.

### Added — Phase 10 (adversarial closeout — logging, trace propagation, spec correctness)

- **F1 — trace-aware logging everywhere**: every log emission inside a
  request- or worker-scoped function now routes through
  `observability.FromContext(ctx)` so `trace_id`, `span_id`, and
  `request_id` are auto-attached. Closed gaps in
  `internal/handler/payment.go`, `internal/worker/organic_canceler.go`,
  `internal/repository/pickup.go` (6 sites), `internal/repository/payment.go`
  (4 sites), and `internal/storage/s3.go` (2 sites); `mapError` default 500
  branch now logs the original error.
- **F2 — W3C trace-context propagator**: registered
  `propagation.NewCompositeTextMapPropagator(TraceContext{}, Baggage{})` so
  inbound `traceparent` headers are honoured and outbound calls propagate
  them. Previously, distributed trace correlation was silently dropped.
- **F3 — outbound S3 calls traced**: MinIO/S3 HTTP client wrapped with
  `otelhttp.NewTransport`, so each `PutObject` appears as a child span.
- **F4 — cross-household payment guard**: `PaymentService.Create` now looks
  up the pickup via `PickupRepository.FindByID` and returns 422
  `BUSINESS_RULE_VIOLATION` if `pickup.HouseholdID ≠ req.HouseholdID`.
- **F5 — self-hosted Swagger UI**: `/api/docs` now serves fully embedded
  Swagger UI (5 vendored dist files via `embed.FS`); no external
  network dependency.
- **F6 — MIME allowlist tightened**: removed `image/jpg` (not IANA); kept
  `image/jpeg`, `image/png`, `application/pdf`.
- **F7 — 429 / 413 trace meta**: rate-limiter 429 response and
  framework-level 413 body-limit response both carry
  `meta.{trace_id, span_id, request_id}` through the unified envelope.
- **F8 — strict pagination validation**: `paginationParams` returns 400
  `VALIDATION_ERROR` on invalid `page` / `per_page` values (non-numeric,
  ≤ 0, or > 100).

### Added — Phase 9 (supply-chain, contract polish, compliance)

- **Tier 8 — supply-chain security**: `govulncheck` CI job, Trivy container
  image scan with HIGH/CRITICAL gate, `SECURITY.md` vulnerability-disclosure
  policy, `.github/CODEOWNERS`, GitHub issue and PR templates,
  `.editorconfig`.
- **Tier 9 — API contract polish**: `pm.test` assertions on every Postman
  request; exhaustive OpenAPI `example:` blocks (planned, fully delivered in
  Phase 11).
- **Tier 10 — realistic load testing**: k6 `smoke`, `average`, `stress`, and
  `spike` scenarios under `test/load/`; SLO thresholds (p95 < 300 ms,
  p99 < 800 ms, error rate < 1%); `make load` target; Grafana load-correlation
  row.
- **Tier 12 — compliance**: `scripts/compliance_check.sh` greps the tree for
  any company-name / email / phone / address leak; `make compliance-check`
  target; CI lint job runs it.

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
