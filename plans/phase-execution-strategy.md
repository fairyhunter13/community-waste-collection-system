# Phase Execution Strategy

## Purpose

Define how to move through all 6 phases: sequencing, phase gates (definition of done), daily commit targets, risk register, and quality checkpoints. This document is the project management layer on top of the technical phase plans.

---

## 1. Phase Overview

| # | Phase | Key Outcome | Dependencies |
|---|---|---|---|
| 0 | Requirements & Analysis | Unambiguous spec — entities, rules, API contract | None |
| 1 | Architecture & Design | Technical structure, interfaces, ADRs | Phase 0 |
| 2 | Infrastructure & DevOps | `make docker-up && make migrate-up` works | Phase 1 |
| 3 | Development | All 15 endpoints functional | Phases 1 + 2 |
| 4 | QA & Testing | All test levels pass, coverage ≥ 70% | Concurrent with Phase 3 |
| 5 | Documentation & Delivery | README + collection + checklist complete | Ongoing from Phase 3 |

---

## 2. Sequencing Rules

```
Phase 0 → must be COMPLETE before Phase 1 starts
Phase 1 → must be COMPLETE before Phase 2 and Phase 3 start
Phase 2 → must be COMPLETE before Phase 3 starts
Phase 3 → sub-phases are sequential; tests written per sub-phase
Phase 4 → NOT a separate block at the end; concurrent with Phase 3
Phase 5 → begins during Phase 3; finalized after Phase 4 passes
```

**Critical insight:** Phase 4 (testing) is embedded in Phase 3. Unit tests are written in the same commit as the code they test. Integration tests follow each repository sub-phase completion. E2E tests are written after handler sub-phase completion.

```
Phase 3 sub-phase → Unit tests → Integration tests (where applicable) → Commit
```

---

## 3. Phase Gates (Definition of Done)

A phase is DONE only when all its gate criteria pass. Do not move to the next phase until the current gate is fully cleared.

### Phase 0 Gate

- [ ] All 3 entities documented with every field: type, constraints, invariants
- [ ] All 6 business rules documented with edge cases and failure conditions
- [ ] All 15 endpoint contracts defined: method, path, request schema, response schema, all error codes
- [ ] Non-functional requirements documented (rate limits, timeouts, file limits, pool sizes)
- [ ] Domain glossary written
- [ ] Plans file committed: `plans/phase-0-requirements-analysis.md`

### Phase 1 Gate

- [ ] Complete project directory structure documented (no ambiguity about where any file goes)
- [ ] All repository interfaces defined with every method signature
- [ ] All service interfaces defined with every method signature
- [ ] Domain error taxonomy complete with HTTP status mapping
- [ ] All 11 ADRs written with context, decision, and consequences
- [ ] Go 1.26.4 features identified with specific application in code
- [ ] Concurrency pattern selected for each use case (worker, fan-out, rate limit, shutdown)
- [ ] Observability architecture documented (OTel spans per layer, Prometheus metrics list)
- [ ] Plans file committed: `plans/phase-1-architecture-design.md`

### Phase 2 Gate

- [ ] `docker build -f build/Dockerfile .` completes successfully
- [ ] `make docker-up` starts all 4 services and all report `healthy`
- [ ] `make migrate-up` applies all 3 migrations in sequence
- [ ] `make migrate-down` reverts all migrations cleanly
- [ ] `golangci-lint run ./...` passes on the project skeleton
- [ ] `.env.example` contains every variable the application reads
- [ ] Pre-commit hook installed and working
- [ ] Plans file committed: `plans/phase-2-infrastructure-devops.md`

### Phase 3 Gate (per sub-phase)

| Sub-phase | Gate Criterion |
|---|---|
| 3.1 Foundation | `go build ./...` and `go vet ./...` pass; config loads from .env |
| 3.2 Data layer | All repository integration tests pass against real PostgreSQL |
| 3.3 Storage | Test file uploads to MinIO; URL returned correctly |
| 3.4 Service layer | All business rule unit tests pass (BR-01 through BR-06) |
| 3.5 HTTP layer | All 15 endpoints respond with correct HTTP status codes via curl |
| 3.6 Observability | `/metrics` returns data; trace appears in OTel collector logs |
| 3.7 Worker | Organic canceler starts, logs, and stops cleanly on SIGINT |
| 3.8 Wiring | Full system works: `make docker-up && make migrate-up && make run` |

### Phase 4 Gate

- [ ] `go test -race -count=1 ./...` passes (unit tests, no build tags)
- [ ] `make test-integration` passes (all repository + service integration tests)
- [ ] `make test-e2e` passes (all 15 endpoint E2E scenarios)
- [ ] `make bench` completes without benchmark failures
- [ ] Coverage ≥ 70% overall: `go tool cover -func=coverage.out`
- [ ] Coverage ≥ 80% for `internal/service`
- [ ] No data races detected by race detector

### Phase 5 Gate

- [ ] README setup steps verified on a clean environment (not just the development machine)
- [ ] Postman/Insomnia collection imports and all requests succeed
- [ ] All items on the delivery checklist in phase-5 checked
- [ ] All plan files committed to `plans/` directory
- [ ] No company-identifying information in any committed file
- [ ] `git log --oneline` shows meaningful daily commits throughout the project

---

## 4. Estimated Timeline (5 Calendar Days)

| Day | Primary Focus | Phase Target | Commits Expected |
|---|---|---|---|
| **Day 1** | Phase 0 (plans committed) + Phase 1 (architecture documented) + Phase 2 (Docker, migrations, lint config) | Gates 0, 1, 2 cleared | 4–5 |
| **Day 2** | Phase 3.1 (foundation) + 3.2 (repositories) + Unit tests for domain/service | Sub-phase gates 3.1, 3.2 | 5–6 |
| **Day 3** | Phase 3.3 (storage) + 3.4 (services) + 3.5 (handlers) + unit tests per layer | Sub-phase gates 3.3–3.5 | 5–7 |
| **Day 4** | Phase 3.6 (observability) + 3.7 (worker) + 3.8 (wiring) + integration + E2E tests | Sub-phase gates 3.6–3.8, partial Phase 4 | 4–6 |
| **Day 5** | Full E2E pass, benchmarks, Phase 5 (README + collection) + final lint/review | Phase 4 gate, Phase 5 gate | 3–5 |

**Daily commit minimum:** At least one substantive commit per working day.

**Note:** Observability (Phase 3.6) is built last among the application features. The core API is fully functional before metrics and traces are wired. This avoids OTel/Prometheus complexity blocking feature delivery.

---

## 5. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| OTel/Prometheus adds more complexity than expected | Medium | Medium | Build last (sub-phase 3.6); core API works independently; use simple logging if OTel stalls |
| testcontainers-go slow startup in CI/local | Medium | Low | Cache Docker images; run integration tests in parallel suites; use `start_period` healthcheck |
| MinIO configuration issues in E2E | Low | Medium | Verify MinIO in sub-phase 3.3 before writing E2E tests; unit tests mock storage |
| golangci-lint too strict blocking progress | Low | Low | Start with essential linters; incrementally add strict ones after core features work |
| Go 1.26 feature unavailable in a dependency | Low | Low | Pin dependency versions; check release notes; fall back to Go 1.25 equivalent if needed |
| E2E test flakiness (timing, port conflicts) | Medium | Low | Use testcontainers for E2E DB; fixed ports; `TearDownTest` truncation between tests |
| Graceful shutdown leaves goroutine leak | Low | High | Test SIGINT explicitly; use Go 1.26's goroutine leak detection (GOEXPERIMENT=goroutineleakprofile) |

---

## 6. Quality Checkpoints (per commit)

Before every `git commit`:
```bash
go build ./...              # must compile
go vet ./...                # no vet errors
golangci-lint run ./...     # no lint errors
go test -race -count=1 -short ./...  # unit tests (short mode for speed)
```

Before each sub-phase gate:
```bash
go test -race -count=1 ./...  # all unit tests
```

Before Phase 4 gate (full test suite):
```bash
make test-integration        # integration tests
make test-e2e                # E2E tests
make bench                   # benchmarks
go test -race -coverprofile=coverage.out -coverpkg=./internal/... ./...
go tool cover -func=coverage.out | tail -1  # check total coverage
```

---

## 7. Daily Routine

```
Morning:
  1. Review yesterday's commits (git log --oneline)
  2. Run: go test -race ./... (ensure baseline still passes)
  3. Identify today's sub-phase target

During work:
  4. Implement smallest testable unit
  5. Write unit test immediately after
  6. Run quality checkpoint before committing
  7. Commit with conventional commit message

End of day:
  8. Run: make test + make lint (full quality check)
  9. Ensure at least one commit was made today
  10. Note any blockers for next day
```

---

## 8. Anti-Patterns to Avoid During Execution

| Anti-Pattern | Why Harmful | Correct Approach |
|---|---|---|
| Writing all code before any tests | Bugs multiply; hard to isolate | Write tests per sub-phase |
| Committing broken code "temporarily" | Corrupts commit history; confuses reviewers | Never commit unless `go build` passes |
| Skipping lint before commit | Accumulates technical debt | Pre-commit hook enforces lint |
| Building observability first | Delays core feature delivery | Core features first; observability in 3.6 |
| Sharing DB state between E2E tests | Flaky, order-dependent tests | `TearDownTest` truncation |
| Copy-pasting handler boilerplate | Drift and inconsistency | Extract shared helpers to handler.go |
| Ignoring golangci-lint errors | Unfixed issues grow | Fix lint errors before committing |
| No `context.Context` in DB calls | Cannot cancel; no timeout | All DB methods take `ctx` as first param |
| Hardcoded sleep in tests | Flaky, slow | Use wait strategies or `synctest` |
| Merging untested code to main | Breaks build for reviewers | All tests pass before any merge |

---

## 9. Execution Sequence Summary

```
Day 1:
  [Phase 0] Write and commit requirements analysis
  [Phase 1] Write and commit architecture design
  [Phase 2] Write Dockerfile, docker-compose, migrations, .golangci.yml, Makefile
           → Verify: make docker-up passes; make migrate-up/down work

Day 2:
  [Phase 3.1] go mod init, config, domain types → go build passes
  [Phase 3.2] Repository implementations + integration tests → test-integration passes
  [Phase 4-unit] Unit test foundation for domain errors, config

Day 3:
  [Phase 3.3] S3 storage client → upload test succeeds against MinIO
  [Phase 3.4] Service layer + unit tests for all business rules → test passes
  [Phase 3.5] HTTP handlers + route registration + handler unit tests → all 15 endpoints respond

Day 4:
  [Phase 3.6] slog + Prometheus + OTel → /metrics and traces working
  [Phase 3.7] Organic canceler worker → SIGINT clean shutdown confirmed
  [Phase 3.8] main.go full wiring → make docker-up full system works
  [Phase 4] Integration + E2E tests → all passing

Day 5:
  [Phase 4] Benchmarks, coverage report → ≥70% overall
  [Phase 5] README verified, Postman collection, delivery checklist
  Final: git log --oneline review; push; done
```
