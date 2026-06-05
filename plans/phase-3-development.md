# Phase 3 — Development

## Purpose

Define the implementation blueprint: module setup, coding conventions, all dependencies, and a sequenced sub-phase breakdown. Every sub-phase has a clear deliverable, a defined implementation pattern, and a verification step before moving to the next.

---

## 1. Module Configuration (`go.mod`)

```go
module github.com/fairyhunter13/community-waste-collection-system

go 1.26

// Tool directives (Go 1.24+) — track dev tools in go.mod without polluting go.sum with main module
tool (
    golang.org/x/tools/cmd/goimports
    github.com/golangci/golangci-lint/cmd/golangci-lint
    github.com/golang-migrate/migrate/v4/cmd/migrate
    github.com/vektra/mockery/v2
)

require (
    // ── HTTP ──────────────────────────────────────────────────────────────
    github.com/labstack/echo/v4        v4.13.x
    github.com/labstack/gommon         v0.4.x

    // ── Database ──────────────────────────────────────────────────────────
    github.com/jmoiron/sqlx            v1.4.x
    github.com/lib/pq                  v1.10.x   // PostgreSQL driver (pure Go)
    github.com/golang-migrate/migrate/v4 v4.18.x

    // ── S3 Storage ────────────────────────────────────────────────────────
    github.com/aws/aws-sdk-go-v2                     v1.x.x
    github.com/aws/aws-sdk-go-v2/config              v1.x.x
    github.com/aws/aws-sdk-go-v2/service/s3          v1.x.x
    github.com/aws/aws-sdk-go-v2/credentials         v1.x.x

    // ── Utilities ─────────────────────────────────────────────────────────
    golang.org/x/time                  v0.x.x    // rate.Limiter for rate limiting
    github.com/google/uuid             v1.6.x
    github.com/go-playground/validator/v10 v10.x.x

    // ── Observability ─────────────────────────────────────────────────────
    go.opentelemetry.io/otel                            v1.x.x
    go.opentelemetry.io/otel/trace                      v1.x.x
    go.opentelemetry.io/otel/sdk                        v1.x.x
    go.opentelemetry.io/otel/exporters/otlp/otlphttp    v1.x.x
    go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho v0.x.x
    github.com/prometheus/client_golang                 v1.x.x

    // ── Testing ───────────────────────────────────────────────────────────
    github.com/stretchr/testify                    v1.10.x
    github.com/testcontainers/testcontainers-go    v0.x.x
    github.com/testcontainers/testcontainers-go/modules/postgres v0.x.x
)
```

**Dependency rationale:**
- `lib/pq` — pure Go PostgreSQL driver; `CGO_ENABLED=0` compatible
- `golang.org/x/time` — stdlib-adjacent package for `rate.Limiter`; avoids heavier Redis-based solutions
- `otelecho` — zero-boilerplate OTel integration for Echo handlers

---

## 2. Code Naming Conventions

### Package names
```
handler    repository    service    domain    config    middleware    worker    storage    observability
```
- All lowercase, single word, no underscores, no abbreviations
- Package name = base directory name (Go convention)

### Types and functions

| Construct | Convention | Example |
|---|---|---|
| Exported struct | PascalCase | `WastePickup`, `Household` |
| Unexported struct | camelCase | `pickupService`, `householdRepo` |
| Enum type | PascalCase | `WasteType`, `PickupStatus` |
| Enum constant | PascalCase | `WasteTypeOrganic`, `PickupStatusPending` |
| Error variable | `Err` + PascalCase | `ErrNotFound`, `ErrConflict` |
| Interface | PascalCase + noun | `HouseholdRepository`, `StorageService` |
| Constructor | `New` + TypeName | `NewPickupService(repo, paymentRepo)` |
| Test function | `Test` + FuncName + `_` + Scenario | `TestPickupService_Create_BlockedByPendingPayment` |
| Benchmark | `Benchmark` + FuncName | `BenchmarkPickupRepository_List` |

### File names
```
household.go      pickup.go       payment.go       report.go
household_test.go pickup_test.go  payment_test.go  report_test.go
```
- File name matches the primary type or feature it contains
- Test files are colocated with the source file

---

## 3. Import Grouping (enforced by goimports)

Three distinct groups separated by blank lines:

```go
import (
    // Group 1: stdlib
    "context"
    "errors"
    "fmt"
    "io"
    "net/http"
    "time"

    // Group 2: external dependencies
    "github.com/google/uuid"
    "github.com/labstack/echo/v4"
    "github.com/jmoiron/sqlx"

    // Group 3: internal packages (identified by module prefix)
    "github.com/fairyhunter13/community-waste-collection-system/internal/domain"
    "github.com/fairyhunter13/community-waste-collection-system/internal/service"
)
```

`goimports -local github.com/fairyhunter13/community-waste-collection-system` enforces this grouping automatically.

---

## 4. Comment Style

- All exported types, functions, constants, and methods **must** have a godoc comment
- Comments are complete sentences ending with a period
- Comment describes WHAT the symbol does, not HOW
- Do NOT add comments explaining obvious code
- Do NOT reference task numbers, PR numbers, or caller names in code comments

```go
// WastePickup represents a household's request to have waste collected on a specific date.
type WastePickup struct {
    ID          uuid.UUID    `db:"id"          json:"id"`
    HouseholdID uuid.UUID    `db:"household_id" json:"household_id"`
    Type        WasteType    `db:"type"        json:"type"`
    Status      PickupStatus `db:"status"      json:"status"`
    PickupDate  *time.Time   `db:"pickup_date" json:"pickup_date"`
    SafetyCheck bool         `db:"safety_check" json:"safety_check"`
    CreatedAt   time.Time    `db:"created_at"  json:"created_at"`
    UpdatedAt   time.Time    `db:"updated_at"  json:"updated_at"`
}

// Complete transitions the pickup to completed status and atomically creates
// the associated payment record within a single database transaction.
func (s *pickupService) Complete(ctx context.Context, id uuid.UUID) (*domain.WastePickup, error) {
    // ...
}
```

---

## 5. Sub-Phase Implementation Order

Sub-phases are sequential. Do not start the next until the current one's verification step passes.

---

### Sub-phase 3.1 — Foundation

**Goal:** Compilable project skeleton with config loading and DB connection.
**Deliverable:** `go build ./...` succeeds; config struct loads from `.env`.

**Files to create:**
- `go.mod`, `go.sum` (via `go mod tidy`)
- `internal/config/config.go` — all env var parsing
- `internal/domain/errors.go` — sentinel errors
- `internal/domain/household.go` — entity + interfaces
- `internal/domain/pickup.go` — entity + enums + interfaces
- `internal/domain/payment.go` — entity + enums + interfaces
- `internal/repository/db.go` — DB connection with pool settings
- `cmd/api/main.go` — skeleton: load config, connect DB, setup signal handling

**Config struct pattern:**
```go
type Config struct {
    AppPort    string
    AppEnv     string
    DebugPort  string

    DatabaseURL        string
    DBMaxOpenConns     int
    DBMaxIdleConns     int
    DBConnMaxIdleTime  time.Duration

    S3Endpoint     string
    S3Bucket       string
    S3AccessKey    string
    S3SecretKey    string
    S3Region       string
    S3UsePathStyle bool

    MaxUploadSizeMB int

    RateLimitRPS   float64
    RateLimitBurst int

    LogLevel   string
    LogFormat  string
    MetricsPort string
    OTELEndpoint    string
    OTELServiceName string

    WorkerCancelInterval   time.Duration
    WorkerOrganicCutoffDays int
}

func Load() *Config {
    return &Config{
        AppPort:   getEnv("APP_PORT", "8080"),
        // ... all fields with defaults
    }
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}
```

**DB connection pattern:**
```go
func Connect(cfg *config.Config) *sqlx.DB {
    db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
    if err != nil {
        log.Fatalf("db connect: %v", err)
    }
    db.SetMaxOpenConns(cfg.DBMaxOpenConns)
    db.SetMaxIdleConns(cfg.DBMaxIdleConns)
    db.SetConnMaxIdleTime(cfg.DBConnMaxIdleTime)
    return db
}
```

**Verification:** `go build ./...` and `go vet ./...` pass.

---

### Sub-phase 3.2 — Data Layer

**Goal:** All migrations applied; all three repository implementations functional.
**Deliverable:** Repository integration tests pass against real PostgreSQL.

**Files to create:**
- `migrations/` — all 6 migration files (as defined in Phase 2)
- `internal/repository/household.go`
- `internal/repository/pickup.go`
- `internal/repository/payment.go`

**sqlx query patterns:**

```go
// Named INSERT (uses struct field names from `db` tag)
result, err := r.db.NamedExecContext(ctx,
    `INSERT INTO households (owner_name, address) VALUES (:owner_name, :address)`,
    h,
)

// SELECT single row into struct
var h domain.Household
err := r.db.GetContext(ctx, &h,
    `SELECT * FROM households WHERE id = $1`,
    id,
)
if errors.Is(err, sql.ErrNoRows) {
    return nil, fmt.Errorf("household %s: %w", id, domain.ErrNotFound)
}

// SELECT multiple rows into slice
var pickups []domain.WastePickup
err := r.db.SelectContext(ctx, &pickups, query, args...)

// Transaction pattern (for pickup complete + payment create atomically)
tx, err := r.db.BeginTxx(ctx, nil)
if err != nil {
    return fmt.Errorf("begin tx: %w", domain.ErrInternalFailure)
}
defer func() {
    if p := recover(); p != nil {
        _ = tx.Rollback()
        panic(p)
    } else if err != nil {
        _ = tx.Rollback()
    }
}()

// ... DB operations using tx ...

err = tx.Commit()
```

**Enum scanning:** PostgreSQL ENUMs scan directly into Go string types with `lib/pq`.

**NUMERIC(12,2) scanning:** Scan into `string` then parse, OR use `shopspring/decimal` if added as dependency, OR store as integer cents (`int64`) in Go.

**Verification:** Integration tests pass with testcontainers-go PostgreSQL.

---

### Sub-phase 3.3 — Storage Layer

**Goal:** S3-compatible file upload working against local MinIO.
**Deliverable:** `Upload()` method successfully puts an object to MinIO and returns its URL.

**File:** `internal/storage/s3.go`

```go
type S3Client struct {
    client   *s3.Client
    bucket   string
    endpoint string
}

func NewS3Client(cfg *config.Config) *S3Client {
    resolver := aws.EndpointResolverWithOptionsFunc(
        func(service, region string, options ...interface{}) (aws.Endpoint, error) {
            return aws.Endpoint{
                URL:               cfg.S3Endpoint,
                HostnameImmutable: cfg.S3UsePathStyle,
            }, nil
        },
    )
    awsCfg, _ := awsconfig.LoadDefaultConfig(context.Background(),
        awsconfig.WithRegion(cfg.S3Region),
        awsconfig.WithEndpointResolverWithOptions(resolver),
        awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
            cfg.S3AccessKey, cfg.S3SecretKey, "",
        )),
    )
    return &S3Client{
        client:   s3.NewFromConfig(awsCfg, func(o *s3.Options) {
            o.UsePathStyle = cfg.S3UsePathStyle
        }),
        bucket:   cfg.S3Bucket,
        endpoint: cfg.S3Endpoint,
    }
}

// Upload uploads a file to S3-compatible storage and returns the public URL.
func (c *S3Client) Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
    _, err := c.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket:        aws.String(c.bucket),
        Key:           aws.String(key),
        Body:          r,
        ContentLength: aws.Int64(size),
        ContentType:   aws.String(contentType),
    })
    if err != nil {
        return "", fmt.Errorf("s3 upload %s: %w", key, domain.ErrInternalFailure)
    }
    return fmt.Sprintf("%s/%s/%s", c.endpoint, c.bucket, key), nil
}
```

**Verification:** Upload a test file via `mc` CLI or the MinIO console; confirm it appears in the bucket.

---

### Sub-phase 3.4 — Service Layer

**Goal:** All business rules (BR-01 through BR-06) implemented and covered by unit tests.
**Deliverable:** Service unit tests pass; business rules enforced correctly.

**Files:** `internal/service/household.go`, `pickup.go`, `payment.go`, `report.go`

**BR-01 implementation (pickup service):**
```go
func (s *pickupService) Create(ctx context.Context, req domain.CreatePickupRequest) (*domain.WastePickup, error) {
    hasPending, err := s.pickupRepo.HasPendingPaymentForHousehold(ctx, req.HouseholdID)
    if err != nil {
        return nil, err
    }
    if hasPending {
        return nil, fmt.Errorf("household has a pending payment: %w", domain.ErrConflict)
    }
    pickup := &domain.WastePickup{
        HouseholdID: req.HouseholdID,
        Type:        req.Type,
        Status:      domain.PickupStatusPending,
        SafetyCheck: req.SafetyCheck,
    }
    if err := s.pickupRepo.Create(ctx, pickup); err != nil {
        return nil, err
    }
    return pickup, nil
}
```

**BR-05 implementation (atomic complete + payment):**
```go
func (s *pickupService) Complete(ctx context.Context, id uuid.UUID) (*domain.WastePickup, error) {
    pickup, err := s.pickupRepo.FindByID(ctx, id)
    if err != nil {
        return nil, err
    }
    if pickup.Status != domain.PickupStatusScheduled {
        return nil, fmt.Errorf("pickup status is %s, must be scheduled: %w", pickup.Status, domain.ErrConflict)
    }

    amount := domain.PaymentAmounts[pickup.Type]
    payment := &domain.Payment{
        HouseholdID: pickup.HouseholdID,
        WasteID:     pickup.ID,
        Amount:      amount,
        Status:      domain.PaymentStatusPending,
    }

    // Atomic: update pickup + create payment in single transaction
    tx, err := s.db.BeginTxx(ctx, nil)
    if err != nil {
        return nil, fmt.Errorf("begin tx: %w", domain.ErrInternalFailure)
    }
    defer func() {
        if err != nil {
            tx.Rollback()
        }
    }()

    if err = s.pickupRepo.UpdateStatusTx(ctx, tx, id, domain.PickupStatusCompleted); err != nil {
        return nil, err
    }
    if err = s.paymentRepo.CreateWithTx(ctx, tx, payment); err != nil {
        return nil, err
    }
    if err = tx.Commit(); err != nil {
        return nil, fmt.Errorf("commit: %w", domain.ErrInternalFailure)
    }

    pickup.Status = domain.PickupStatusCompleted
    return pickup, nil
}
```

**Report service fan-out pattern (parallel DB queries):**
```go
func (s *reportService) WasteSummary(ctx context.Context) ([]domain.WasteTypeSummary, error) {
    // Single query with GROUP BY handles aggregation efficiently — no fan-out needed
    return s.paymentRepo.WasteSummary(ctx)
}
```

**Verification:** `make test-unit` passes with ≥80% service coverage.

---

### Sub-phase 3.5 — HTTP Layer

**Goal:** All 15 endpoints registered and returning correct status codes.
**Deliverable:** `curl` against each endpoint returns the expected shape.

**Files:** `internal/handler/handler.go`, `household.go`, `pickup.go`, `payment.go`, `report.go`

**Handler struct and route registration:**
```go
type Handler struct {
    householdSvc domain.HouseholdService
    pickupSvc    domain.PickupService
    paymentSvc   domain.PaymentService
    reportSvc    domain.ReportService
    validate     *validator.Validate
    cfg          *config.Config
}

func New(hSvc domain.HouseholdService, pSvc domain.PickupService,
    paymentSvc domain.PaymentService, rSvc domain.ReportService,
    cfg *config.Config) *Handler {
    return &Handler{
        householdSvc: hSvc,
        pickupSvc:    pSvc,
        paymentSvc:   paymentSvc,
        reportSvc:    rSvc,
        validate:     validator.New(),
        cfg:          cfg,
    }
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
    api := e.Group("/api")

    // Households
    api.POST("/households",     h.CreateHousehold)
    api.GET("/households",      h.ListHouseholds)
    api.GET("/households/:id",  h.GetHousehold)
    api.DELETE("/households/:id", h.DeleteHousehold)

    // Pickups — rate limiter applied only here
    pickups := api.Group("/pickups")
    pickups.POST("", h.CreatePickup, middleware.RateLimiter(h.cfg))
    pickups.GET("", h.ListPickups)
    pickups.PUT("/:id/schedule", h.SchedulePickup)
    pickups.PUT("/:id/complete", h.CompletePickup)
    pickups.PUT("/:id/cancel",   h.CancelPickup)

    // Payments
    api.POST("/payments",            h.CreatePayment)
    api.GET("/payments",             h.ListPayments)
    api.PUT("/payments/:id/confirm", h.ConfirmPayment)

    // Reports
    reports := api.Group("/reports")
    reports.GET("/waste-summary",            h.WasteSummary)
    reports.GET("/payment-summary",          h.PaymentSummary)
    reports.GET("/households/:id/history",   h.HouseholdHistory)
}
```

**Standard handler pattern:**
```go
func (h *Handler) CreatePickup(c echo.Context) error {
    var req domain.CreatePickupRequest
    if err := c.Bind(&req); err != nil {
        return h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
    }
    if err := h.validate.StructCtx(c.Request().Context(), req); err != nil {
        return h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
    }
    pickup, err := h.pickupSvc.Create(c.Request().Context(), req)
    if err != nil {
        return h.mapError(c, err)
    }
    return c.JSON(http.StatusCreated, successResponse(pickup))
}
```

**File upload handler (payment confirm):**
```go
func (h *Handler) ConfirmPayment(c echo.Context) error {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        return h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid id")
    }

    file, err := c.FormFile("proof")
    if err != nil {
        return h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "proof file is required")
    }
    if file.Size > int64(h.cfg.MaxUploadSizeMB)*1024*1024 {
        return h.respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "file too large")
    }

    src, err := file.Open()
    if err != nil {
        return h.respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to open file")
    }
    defer src.Close()

    payment, err := h.paymentSvc.Confirm(c.Request().Context(), id, src, file.Size, file.Header.Get("Content-Type"))
    if err != nil {
        return h.mapError(c, err)
    }
    return c.JSON(http.StatusOK, successResponse(payment))
}
```

**Verification:** All 15 endpoints respond with correct status codes via curl or Postman.

---

### Sub-phase 3.6 — Observability

**Goal:** Metrics at `:2112/metrics`, traces in OTel collector, pprof at `:6060`.
**Deliverable:** All three observability endpoints return data.

**Files:** `internal/observability/logger.go`, `metrics.go`, `tracer.go`

**slog logger setup:**
```go
func NewLogger(cfg *config.Config) *slog.Logger {
    var level slog.Level
    _ = level.UnmarshalText([]byte(cfg.LogLevel))

    opts := &slog.HandlerOptions{Level: level}
    var handler slog.Handler
    if cfg.LogFormat == "json" {
        handler = slog.NewJSONHandler(os.Stdout, opts)
    } else {
        handler = slog.NewTextHandler(os.Stdout, opts)
    }
    return slog.New(handler)
}
```

**OTel tracer setup:**
```go
func InitTracer(cfg *config.Config) (trace.Tracer, func(context.Context)) {
    exporter, _ := otlphttp.New(context.Background(),
        otlphttp.WithEndpoint(cfg.OTELEndpoint),
        otlphttp.WithInsecure(),
    )
    provider := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName(cfg.OTELServiceName),
        )),
    )
    otel.SetTracerProvider(provider)
    return provider.Tracer(cfg.OTELServiceName), provider.Shutdown
}
```

**pprof setup in main.go:**
```go
// Start debug server on separate port (never the same port as the API)
go func() {
    debugMux := http.NewServeMux()
    debugMux.Handle("/debug/pprof/", http.DefaultServeMux)
    if err := http.ListenAndServe(":"+cfg.DebugPort, nil); err != nil {
        logger.Error("debug server error", "error", err)
    }
}()
```

**Verification:** `curl http://localhost:2112/metrics` returns Prometheus data; docker logs for otel-collector show incoming spans.

---

### Sub-phase 3.7 — Background Worker

**Goal:** Organic auto-cancel goroutine running, logging cancellations, stopping cleanly on SIGINT.
**Deliverable:** SIGINT causes clean log line "organic canceler stopping" within 10s.

**File:** `internal/worker/organic_canceler.go`

```go
// OrganicCanceler monitors pending organic pickups and cancels those older than
// the configured cutoff duration.
type OrganicCanceler struct {
    repo     domain.PickupRepository
    logger   *slog.Logger
    interval time.Duration
    cutoff   time.Duration
    counter  prometheus.Counter  // worker_organic_cancels_total
}

// NewOrganicCanceler constructs a canceler with the given configuration.
func NewOrganicCanceler(repo domain.PickupRepository, logger *slog.Logger, cfg *config.Config, counter prometheus.Counter) *OrganicCanceler {
    return &OrganicCanceler{
        repo:     repo,
        logger:   logger,
        interval: cfg.WorkerCancelInterval,
        cutoff:   time.Duration(cfg.WorkerOrganicCutoffDays) * 24 * time.Hour,
        counter:  counter,
    }
}

// Start runs the cancellation loop until ctx is cancelled.
// It is designed to be run as a goroutine.
func (w *OrganicCanceler) Start(ctx context.Context) {
    w.logger.Info("organic canceler started", "interval", w.interval, "cutoff", w.cutoff)
    ticker := time.NewTicker(w.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            w.logger.Info("organic canceler stopping")
            return
        case <-ticker.C:
            w.run(ctx)
        }
    }
}

func (w *OrganicCanceler) run(ctx context.Context) {
    before := time.Now().Add(-w.cutoff)
    pickups, err := w.repo.FindExpiredOrganic(ctx, before)
    if err != nil {
        w.logger.Error("find expired organic pickups", "error", err)
        return
    }
    if len(pickups) == 0 {
        return
    }
    ids := make([]uuid.UUID, len(pickups))
    for i, p := range pickups {
        ids[i] = p.ID
    }
    if err := w.repo.BulkCancel(ctx, ids); err != nil {
        w.logger.Error("bulk cancel organic pickups", "error", err, "count", len(ids))
        return
    }
    w.logger.Info("organic pickups auto-canceled", "count", len(ids))
    w.counter.Add(float64(len(ids)))
}
```

**Verification:** Start the app, send SIGINT (`Ctrl+C`), observe clean shutdown log within 10s.

---

### Sub-phase 3.8 — Main Wiring & Graceful Shutdown

**Goal:** Single binary, fully wired, clean shutdown sequence.
**Deliverable:** `make docker-up && make migrate-up` → full working system.

**File:** `cmd/api/main.go`

```go
func main() {
    cfg := config.Load()

    // ── Observability ──────────────────────────────────────────────────────
    logger := observability.NewLogger(cfg)
    slog.SetDefault(logger)

    tracer, shutdownTracer := observability.InitTracer(cfg)
    defer shutdownTracer(context.Background())

    // ── Database ───────────────────────────────────────────────────────────
    db := repository.Connect(cfg)
    defer db.Close()

    // ── Repositories ───────────────────────────────────────────────────────
    householdRepo := repository.NewHouseholdRepository(db)
    pickupRepo    := repository.NewPickupRepository(db)
    paymentRepo   := repository.NewPaymentRepository(db)

    // ── Storage ────────────────────────────────────────────────────────────
    storageClient := storage.NewS3Client(cfg)

    // ── Services ───────────────────────────────────────────────────────────
    householdSvc := service.NewHouseholdService(householdRepo)
    pickupSvc    := service.NewPickupService(pickupRepo, paymentRepo, db)
    paymentSvc   := service.NewPaymentService(paymentRepo, storageClient)
    reportSvc    := service.NewReportService(pickupRepo, paymentRepo, householdRepo)

    // ── HTTP ───────────────────────────────────────────────────────────────
    e := echo.New()
    e.HideBanner = true
    e.Use(otelecho.Middleware(cfg.OTELServiceName))
    e.Use(middleware.Logger(logger))
    e.Use(middleware.Recover(logger))
    e.Use(echomiddleware.CrossOriginProtection())  // Go 1.25

    h := handler.New(householdSvc, pickupSvc, paymentSvc, reportSvc, cfg)
    h.RegisterRoutes(e)

    // ── Debug server (pprof) ───────────────────────────────────────────────
    go func() {
        logger.Info("pprof server starting", "port", cfg.DebugPort)
        if err := http.ListenAndServe(":"+cfg.DebugPort, nil); err != nil {
            logger.Error("pprof server error", "error", err)
        }
    }()

    // ── Metrics server ─────────────────────────────────────────────────────
    go observability.StartMetricsServer(cfg.MetricsPort, logger)

    // ── Background worker ──────────────────────────────────────────────────
    workerCtx, workerCancel := context.WithCancel(context.Background())
    var wg sync.WaitGroup
    wg.Add(1)
    go func() {
        defer wg.Done()
        worker.NewOrganicCanceler(pickupRepo, logger, cfg, observability.OrganicCancelsTotal).Start(workerCtx)
    }()

    // ── API server ─────────────────────────────────────────────────────────
    go func() {
        logger.Info("API server starting", "port", cfg.AppPort)
        if err := e.Start(":" + cfg.AppPort); err != nil && !errors.Is(err, http.ErrServerClosed) {
            logger.Error("API server error", "error", err)
        }
    }()

    // ── Graceful shutdown ──────────────────────────────────────────────────
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    sig := <-quit
    logger.Info("shutdown signal received", "signal", sig)

    // 1. Stop accepting new requests
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer shutdownCancel()
    if err := e.Shutdown(shutdownCtx); err != nil {
        logger.Error("server shutdown error", "error", err)
    }

    // 2. Stop background worker
    workerCancel()
    wg.Wait()

    logger.Info("shutdown complete")
}
```

**Verification:**
1. `make docker-up && make migrate-up && make run` — server starts
2. SIGINT → clean "shutdown complete" log within 10s
3. All endpoints respond correctly

---

## 6. Anti-Patterns to Avoid

| Anti-Pattern | Consequence | Correct Approach |
|---|---|---|
| Business logic in handlers | Untestable without HTTP | Move to service layer |
| `interface{}` in request/response | Loses compile-time safety | Use typed structs |
| `log.Fatal` in library code | Prevents graceful shutdown | Return error to caller |
| Goroutine without exit condition | Goroutine leak | Always use context or done channel |
| `float64` for money | Precision errors (`0.1+0.2 != 0.3`) | `NUMERIC(12,2)` in DB; integer cents in Go |
| String concatenation in SQL | SQL injection | Parameterized queries (`$1`, NamedExec) |
| Global mutable state | Race conditions | Inject all state via constructor |
| Returning `200 OK` for errors | Breaks API contract | Always use correct HTTP status codes |
| Swallowing errors with `_` | Silent failures | Always handle or explicitly ignore with comment |
| Large functions (>50 lines) | Poor readability | Extract into named helper functions |
