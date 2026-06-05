# Phase 1 — Architecture & Design

## Purpose

Define the technical structure, layer contracts, interface shapes, error taxonomy, observability strategy, concurrency patterns, and key Go principles — before writing any implementation code. Every implementation decision in Phase 3 traces back to a decision made here.

---

## 1. Project Structure (Golang Standard Layout)

The project follows the community Go standard project layout. Directories not used by this project are omitted rather than left empty.

```
community-waste-collection-system/
│
├── cmd/
│   └── api/
│       └── main.go                    # Entry point. Wire all dependencies. Start server.
│
├── internal/                          # Compiler-enforced private code (Go 1.4+)
│   │
│   ├── config/
│   │   └── config.go                  # Config struct; all values from env vars
│   │
│   ├── domain/                        # Pure domain layer — no imports from other internal packages
│   │   ├── errors.go                  # Sentinel error variables
│   │   ├── household.go               # Household struct + HouseholdRepository/Service interfaces
│   │   ├── pickup.go                  # WastePickup struct + enums + interfaces
│   │   └── payment.go                 # Payment struct + enums + interfaces
│   │
│   ├── handler/                       # HTTP handlers (Echo). No business logic.
│   │   ├── handler.go                 # Handler struct; route registration; response helpers
│   │   ├── household.go
│   │   ├── pickup.go
│   │   ├── payment.go
│   │   └── report.go
│   │
│   ├── service/                       # Business logic. Depends on repository interfaces only.
│   │   ├── household.go
│   │   ├── pickup.go
│   │   ├── payment.go
│   │   └── report.go
│   │
│   ├── repository/                    # sqlx implementations of repository interfaces
│   │   ├── db.go                      # DB connection helper (sqlx.Connect + pool config)
│   │   ├── household.go
│   │   ├── pickup.go
│   │   └── payment.go
│   │
│   ├── middleware/
│   │   ├── ratelimit.go               # Per-IP token bucket; per-route on POST /api/pickups
│   │   ├── logger.go                  # slog request/response middleware
│   │   ├── trace.go                   # OTel span propagation (via otelecho)
│   │   └── recover.go                 # Panic recovery → structured 500 response
│   │
│   ├── worker/
│   │   └── organic_canceler.go        # Background goroutine; ticker loop; context-cancellable
│   │
│   ├── storage/
│   │   └── s3.go                      # S3-compatible client wrapper (aws-sdk-go-v2)
│   │
│   └── observability/
│       ├── logger.go                  # slog setup: JSONHandler, level from config
│       ├── metrics.go                 # Prometheus metric definitions and registration
│       └── tracer.go                  # OTel SDK init, OTLP exporter, global tracer
│
├── api/
│   └── community-waste.postman_collection.json
│
├── build/
│   └── Dockerfile                     # Multi-stage: golang:1.26-alpine → alpine:3.22
│
├── deployments/
│   ├── docker-compose.yml             # app + postgres + minio + otel-collector
│   └── otel-collector-config.yaml
│
├── migrations/
│   ├── 000001_create_households.up.sql
│   ├── 000001_create_households.down.sql
│   ├── 000002_create_pickups.up.sql
│   ├── 000002_create_pickups.down.sql
│   ├── 000003_create_payments.up.sql
│   └── 000003_create_payments.down.sql
│
├── plans/                             # Engineering phase documents (this directory)
│
├── scripts/
│   └── seed.sql                       # Optional: seed data for local development
│
├── test/
│   └── e2e/                           # External E2E test suite
│       ├── suite_test.go
│       ├── household_test.go
│       ├── pickup_test.go
│       ├── payment_test.go
│       └── report_test.go
│
├── .golangci.yml
├── .env.example
├── docker-compose.yml                 # Root-level symlink to deployments/docker-compose.yml
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### Directory Rationale

| Directory | Why It Exists |
|---|---|
| `/cmd/api/` | Standard location for executable entry points; kept minimal (wiring only) |
| `/internal/` | Go compiler prevents external packages from importing this code |
| `/internal/domain/` | Zero dependencies on other internal packages; entities and interfaces only |
| `/internal/handler/` | HTTP concern only — bind, validate, call service, format response |
| `/internal/service/` | Business logic; depends only on domain interfaces (testable with mocks) |
| `/internal/repository/` | Data access; implements domain repository interfaces with sqlx |
| `/api/` | Non-Go API artifacts (Postman collection, future OpenAPI spec) |
| `/build/` | Build tooling separated from application source |
| `/deployments/` | Orchestration config (docker-compose, OTel collector) |
| `/test/e2e/` | External test apps per golang-standards guidance |
| `/plans/` | Engineering documentation committed alongside code |

---

## 2. Layer Architecture

```
HTTP Request
     │
     ▼
┌─────────────────────────────────────────┐
│           Echo Router                   │
│  Middleware: logger → trace → recover   │
│  Route-level: rate limiter (pickups)    │
└──────────────────┬──────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────┐
│              Handler Layer              │
│  • Bind request body                    │
│  • Validate with go-playground/validator│
│  • Call service method                  │
│  • Map domain error → HTTP status       │
│  • Return JSON envelope                 │
│  NO business logic allowed here         │
└──────────────────┬──────────────────────┘
                   │ (domain interfaces)
                   ▼
┌─────────────────────────────────────────┐
│              Service Layer              │
│  • Enforce all business rules (BR-01–06)│
│  • Orchestrate multiple repository calls│
│  • Own transaction boundaries           │
│  • Emit OTel spans                      │
│  Depends on: domain interfaces only     │
└──────────────────┬──────────────────────┘
                   │ (domain interfaces)
                   ▼
┌─────────────────────────────────────────┐
│           Repository Layer              │
│  • Execute SQL via sqlx                 │
│  • Map rows to domain structs           │
│  • Wrap DB errors with domain errors    │
│  Depends on: *sqlx.DB                   │
└──────────────────┬──────────────────────┘
                   │
                   ▼
             PostgreSQL 17
```

**Layer rules (enforced by package boundaries):**
- Handlers never import `repository` packages directly
- Services never import `handler` packages
- `domain` package imports nothing from `internal/` (no cycles)
- All cross-layer calls go through interfaces defined in `domain/`
- `context.Context` is the first parameter on every method that can block

---

## 3. Interface Contracts

All interfaces are defined in `internal/domain/`. Implementations live in `internal/repository/` and `internal/service/`.

### Repository Interfaces

```go
// internal/domain/household.go
type HouseholdRepository interface {
    Create(ctx context.Context, h *Household) error
    FindByID(ctx context.Context, id uuid.UUID) (*Household, error)
    List(ctx context.Context, page, perPage int) ([]*Household, int, error)
    Delete(ctx context.Context, id uuid.UUID) error
}

// internal/domain/pickup.go
type PickupRepository interface {
    Create(ctx context.Context, p *WastePickup) error
    FindByID(ctx context.Context, id uuid.UUID) (*WastePickup, error)
    List(ctx context.Context, filter PickupFilter) ([]*WastePickup, int, error)
    UpdateStatus(ctx context.Context, id uuid.UUID, status PickupStatus, tx ...*sqlx.Tx) error
    Schedule(ctx context.Context, id uuid.UUID, date time.Time) error
    FindExpiredOrganic(ctx context.Context, before time.Time) ([]*WastePickup, error)
    BulkCancel(ctx context.Context, ids []uuid.UUID) error
    HasPendingPaymentForHousehold(ctx context.Context, householdID uuid.UUID) (bool, error)
}

// internal/domain/payment.go
type PaymentRepository interface {
    Create(ctx context.Context, p *Payment) error
    CreateWithTx(ctx context.Context, tx *sqlx.Tx, p *Payment) error
    FindByID(ctx context.Context, id uuid.UUID) (*Payment, error)
    List(ctx context.Context, filter PaymentFilter) ([]*Payment, int, error)
    Confirm(ctx context.Context, id uuid.UUID, proofURL string, paidAt time.Time) error
    WasteSummary(ctx context.Context) ([]WasteTypeSummary, error)
    PaymentSummary(ctx context.Context) (*PaymentSummaryResult, error)
    HouseholdHistory(ctx context.Context, householdID uuid.UUID) (*HouseholdHistory, error)
}
```

### Service Interfaces

```go
// internal/domain/household.go
type HouseholdService interface {
    Create(ctx context.Context, req CreateHouseholdRequest) (*Household, error)
    GetByID(ctx context.Context, id uuid.UUID) (*Household, error)
    List(ctx context.Context, page, perPage int) ([]*Household, int, error)
    Delete(ctx context.Context, id uuid.UUID) error
}

// internal/domain/pickup.go
type PickupService interface {
    Create(ctx context.Context, req CreatePickupRequest) (*WastePickup, error)
    List(ctx context.Context, filter PickupFilter) ([]*WastePickup, int, error)
    Schedule(ctx context.Context, id uuid.UUID, req SchedulePickupRequest) (*WastePickup, error)
    Complete(ctx context.Context, id uuid.UUID) (*WastePickup, error)
    Cancel(ctx context.Context, id uuid.UUID) (*WastePickup, error)
}

// internal/domain/payment.go
type PaymentService interface {
    Create(ctx context.Context, req CreatePaymentRequest) (*Payment, error)
    List(ctx context.Context, filter PaymentFilter) ([]*Payment, int, error)
    Confirm(ctx context.Context, id uuid.UUID, file io.Reader, size int64, contentType string) (*Payment, error)
}

// internal/domain/report.go (or payment.go)
type ReportService interface {
    WasteSummary(ctx context.Context) ([]WasteTypeSummary, error)
    PaymentSummary(ctx context.Context) (*PaymentSummaryResult, error)
    HouseholdHistory(ctx context.Context, id uuid.UUID) (*HouseholdHistory, error)
}

// internal/domain/storage.go
type StorageService interface {
    Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) (url string, err error)
}
```

### Key Domain Types

```go
// Enums
type WasteType    string
type PickupStatus string
type PaymentStatus string

const (
    WasteTypeOrganic    WasteType = "organic"
    WasteTypePlastic    WasteType = "plastic"
    WasteTypePaper      WasteType = "paper"
    WasteTypeElectronic WasteType = "electronic"
)

const (
    PickupStatusPending   PickupStatus = "pending"
    PickupStatusScheduled PickupStatus = "scheduled"
    PickupStatusCompleted PickupStatus = "completed"
    PickupStatusCanceled  PickupStatus = "canceled"
)

const (
    PaymentStatusPending PaymentStatus = "pending"
    PaymentStatusPaid    PaymentStatus = "paid"
    PaymentStatusFailed  PaymentStatus = "failed"
)

// Payment amounts by waste type
var PaymentAmounts = map[WasteType]decimal.Decimal{
    WasteTypeOrganic:    decimal.NewFromInt(50000),
    WasteTypePlastic:    decimal.NewFromInt(50000),
    WasteTypePaper:      decimal.NewFromInt(50000),
    WasteTypeElectronic: decimal.NewFromInt(100000),
}
```

---

## 4. Domain Error Taxonomy

```go
// internal/domain/errors.go

// Sentinel errors — wrap with context using fmt.Errorf("...: %w", ErrX)
var (
    ErrNotFound        = errors.New("not found")
    ErrConflict        = errors.New("conflict")
    ErrBusinessRule    = errors.New("business rule violation")
    ErrValidation      = errors.New("validation error")
    ErrInternalFailure = errors.New("internal failure")
)
```

**Usage pattern:**
```go
// In repository (wrap with entity context):
return nil, fmt.Errorf("household %s: %w", id, domain.ErrNotFound)

// In service (wrap with rule context):
return nil, fmt.Errorf("pickup creation blocked: %w", domain.ErrConflict)

// In handler (unwrap with errors.Is):
switch {
case errors.Is(err, domain.ErrNotFound):
    return respondError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
case errors.Is(err, domain.ErrConflict):
    return respondError(c, http.StatusConflict, "CONFLICT", err.Error())
case errors.Is(err, domain.ErrBusinessRule):
    return respondError(c, http.StatusUnprocessableEntity, "BUSINESS_RULE_VIOLATION", err.Error())
case errors.Is(err, domain.ErrValidation):
    return respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
default:
    return respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
}
```

**HTTP status mapping:**

| Domain Error | HTTP Status | Error Code |
|---|---|---|
| `ErrNotFound` | 404 | `NOT_FOUND` |
| `ErrConflict` | 409 | `CONFLICT` |
| `ErrBusinessRule` | 422 | `BUSINESS_RULE_VIOLATION` |
| `ErrValidation` | 400 | `VALIDATION_ERROR` |
| `ErrInternalFailure` | 500 | `INTERNAL_ERROR` |
| rate limiter | 429 | `RATE_LIMITED` |

---

## 5. Go 1.26.4 Features Leveraged

| Feature | Go Version | Where Applied |
|---|---|---|
| `log/slog` structured logging | 1.21 | All application logging; replaces zerolog/zap |
| `slog.NewMultiHandler` | 1.26 | Route logs to multiple sinks (stdout + file in production) |
| `errors.AsType[T]` | 1.26 | Type-safe error unwrapping in handlers (alternative to type assertion) |
| `sync.WaitGroup.Go()` | 1.25 | Background worker launch in main.go; cleaner than wg.Add(1) + go |
| `T.Context()` | 1.24 | Test context tied to test lifetime in unit/integration tests |
| `B.Loop()` | 1.24 | Integration benchmark iteration (accurate, no manual b.ResetTimer) |
| `testing.ArtifactDir()` | 1.26 | Save benchmark results to test artifact directory |
| `net/http.CrossOriginProtection` | 1.25 | CSRF protection middleware (applied globally) |
| Range-over-func | 1.23 | Iterator patterns in report aggregation and slice processing |
| Green Tea GC | 1.26 (default) | Automatic — 10-40% GC overhead reduction; no code change needed |
| `new()` with expressions | 1.26 | Cleaner struct initialization where beneficial |
| Container-aware `GOMAXPROCS` | 1.25 | Automatic CPU limit respect in Docker; no manual tuning |

---

## 6. Concurrency Pattern Decision Guide

Choose the pattern that fits the specific use case. There is no single universal pattern.

| Use Case | Pattern | Implementation |
|---|---|---|
| Organic auto-cancel worker | **Worker loop + context cancellation** | Single goroutine with `time.Ticker`; exits when `ctx.Done()` fires |
| Report aggregation (parallel DB queries) | **Fan-out / fan-in** | Launch goroutines for waste + payment summary queries; `sync.WaitGroup.Go()` to merge |
| Rate limiting pickup creation | **Token bucket** | `rate.Limiter` from `golang.org/x/time/rate`; per-IP map protected by `sync.Map` |
| Graceful shutdown | **Context propagation + WaitGroup** | Cancel root ctx → propagate to all goroutines → `wg.Wait()` before Echo shutdown |
| DB connection management | **Built-in pool** | `sqlx.DB` pool; `MaxOpenConns=25`, `MaxIdleConns=10`, `ConnMaxIdleTime=5m` |
| HTTP request handling | **Framework goroutines** | Echo handles one goroutine per request; no custom goroutine management needed |
| Pickup complete + payment create | **Database transaction** | Atomic operation in single TX; no goroutines needed |

**Goroutine lifecycle rules:**
- Every goroutine MUST have a defined exit condition: context cancellation, channel close, or done signal
- Never launch a goroutine without a mechanism to observe its completion
- Use `sync.WaitGroup.Go()` (Go 1.25) over manual `wg.Add(1); go func()`
- Pass `context.Context` as the first parameter to all functions that can block on I/O or goroutines

---

## 7. Observability Architecture

```
Request arrives
     │
     ├── OTel trace middleware → create root span with trace_id
     │        │
     │        ▼
     │   [Handler span: http.method, http.route, http.status_code]
     │        │
     │        ▼
     │   [Service span: business.entity, business.action]
     │        │
     │        ▼
     │   [Repository span: db.statement, db.rows_affected]
     │
     └── slog middleware → structured log: method, path, status, duration, trace_id
```

**Prometheus metrics (exported at `:2112/metrics`):**

| Metric | Type | Labels |
|---|---|---|
| `http_requests_total` | Counter | `method`, `route`, `status_code` |
| `http_request_duration_seconds` | Histogram | `method`, `route` |
| `db_query_duration_seconds` | Histogram | `query_name` |
| `worker_organic_cancels_total` | Counter | — |
| `storage_upload_duration_seconds` | Histogram | — |

**OTel span attributes:**
- Handler: `http.method`, `http.route`, `http.status_code`, `net.peer.ip`
- Service: `business.rule`, `entity.type`, `entity.id`
- Repository: `db.system=postgresql`, `db.operation`, `db.sql.table`

**pprof (exposed at `:6060/debug/pprof/`):**
- CPU profile: `/debug/pprof/profile`
- Heap: `/debug/pprof/heap`
- Goroutines: `/debug/pprof/goroutine`
- Block: `/debug/pprof/block`

---

## 8. Architecture Decision Records (ADRs)

### ADR-001: Echo v4 for HTTP Framework
**Status:** Accepted
**Context:** Go 1.22 enhanced stdlib ServeMux with method routing and path wildcards, reducing the gap between stdlib and frameworks. However, Echo provides middleware composition, built-in request binding with content-type detection, native go-playground/validator integration, and route groups.
**Decision:** Use Echo v4. The validator integration and middleware ergonomics provide value beyond what stdlib offers at this project scale.
**Consequences:** Echo dependency; no lock-in since business logic is framework-agnostic via interfaces.

---

### ADR-002: sqlx + Raw SQL over GORM
**Status:** Accepted
**Context:** GORM abstracts SQL but makes PostgreSQL-specific behavior opaque (ENUM types, UUID, NUMERIC, partial indexes, CTEs). Raw SQL gives full control over query performance.
**Decision:** Use sqlx for struct scanning and named queries. All SQL is written explicitly in repository files.
**Consequences:** More code in repository layer; complete control over query behavior and PostgreSQL-specific features.

---

### ADR-003: Manual Dependency Injection
**Status:** Accepted
**Context:** `google/wire` adds a build step and code generation. `uber/fx` adds runtime container complexity. Both obscure the dependency graph.
**Decision:** Wire all dependencies explicitly in `cmd/api/main.go`. The full dependency graph is readable as plain Go code.
**Consequences:** `main.go` grows as service grows; acceptable for this project size; no magic.

---

### ADR-004: Custom Domain Errors over pkg/errors
**Status:** Accepted
**Context:** `pkg/errors` is largely superseded by stdlib `%w` wrapping since Go 1.13. Stack traces are available via pprof and debug builds.
**Decision:** Sentinel errors in `domain/errors.go`; wrap with context via `fmt.Errorf("...: %w", ErrX)`; unwrap in handlers with `errors.Is`.
**Consequences:** Clean, testable error hierarchy with no third-party dependency.

---

### ADR-005: testcontainers-go for Integration Tests
**Status:** Accepted
**Context:** Shared docker-compose DB creates test isolation problems (dirty data between runs). SQLite is not PostgreSQL-compatible (misses ENUMs, UUID functions, NUMERIC).
**Decision:** Spin up a real PostgreSQL 17 container per test suite using testcontainers-go. Apply migrations before suite runs. Truncate tables between individual tests.
**Consequences:** ~2s overhead per test suite startup; fully isolated and reproducible without manual setup.

---

### ADR-006: log/slog over zerolog/zap
**Status:** Accepted
**Context:** zerolog and zap have excellent performance, but both add third-party dependencies. Go 1.21 introduced `log/slog` with a JSON handler and context propagation. Go 1.26 added `slog.NewMultiHandler`.
**Decision:** Use stdlib `log/slog` with `JSONHandler`. Log at the appropriate level; include `trace_id` in every request log via slog's `With` method.
**Consequences:** Zero dependency; slightly less feature-rich than zerolog, entirely acceptable for this scope.

---

### ADR-007: Prometheus + OpenTelemetry for Observability
**Status:** Accepted
**Context:** Logging alone is insufficient for production-readiness awareness. RED metrics (Rate, Errors, Duration) and distributed traces are standard expectations.
**Decision:** Prometheus for metrics (`/metrics` on port 2112); OTel for traces (OTLP export). Both run in docker-compose alongside the application.
**Consequences:** Additional dependencies; docker-compose includes otel-collector; demonstrates full production observability thinking.

---

### ADR-008: golang-migrate for Migrations
**Status:** Accepted
**Context:** goose supports both SQL and Go migration files; atlas is declarative. golang-migrate uses plain SQL up/down files and integrates cleanly with the library API for programmatic application.
**Decision:** Use golang-migrate with numbered SQL files (`000001_name.up.sql` / `.down.sql`).
**Consequences:** Pure SQL migrations; portable; CLI and library modes; easy to embed in docker-compose startup.

---

### ADR-009: amount as NUMERIC(12,2) not float
**Status:** Accepted
**Context:** Floating point arithmetic produces rounding errors (e.g., `0.1 + 0.2 != 0.3`). Financial values must be exact.
**Decision:** Store `amount` as PostgreSQL `NUMERIC(12,2)`. Represent in Go as a decimal type or as integer cents (50000 = 50,000.00). Never use `float64` for money.
**Consequences:** Requires decimal-aware scanning from sqlx; slight verbosity in Go code; exact arithmetic guaranteed.
