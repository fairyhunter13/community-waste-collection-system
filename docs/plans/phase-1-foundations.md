# Phase 1 — Foundations

Establishes the project skeleton: binary entry point, dependency injection,
configuration loading, graceful shutdown, domain interfaces, and sentinel errors.

## Entry point and DI wiring

`cmd/api/main.go` is the single binary. It:

1. Loads config from environment variables via `config.Load()` (`internal/config/config.go`).
2. Opens the PostgreSQL connection pool (`sqlx.Connect`).
3. Constructs the S3/MinIO client (`storage.NewS3Client`).
4. Instantiates repository implementations, then service implementations, then the HTTP handler.
5. Registers Echo routes via `handler.RegisterRoutes`.
6. Starts the background OrganicCanceler worker in a goroutine.
7. Blocks on `e.Start(cfg.HTTPAddr)` until a SIGINT/SIGTERM triggers graceful shutdown.

## Configuration

`internal/config/config.go` reads all tunables from environment variables with
documented defaults. Notable fields:

| Variable | Default | Purpose |
|----------|---------|---------|
| `HTTP_ADDR` | `:8080` | Echo listen address |
| `DATABASE_URL` | — | PostgreSQL DSN (required) |
| `RATE_LIMIT_RPS` | `10` | Token bucket refill rate per IP |
| `RATE_LIMIT_BURST` | `20` | Token bucket burst capacity |
| `WORKER_CANCEL_INTERVAL` | `1h` | BR-04 organic-canceler tick |
| `WORKER_ORGANIC_CUTOFF_DAYS` | `3` | BR-04 age threshold |
| `WORKER_QUERY_TIMEOUT` | `5s` | Per-query timeout in worker tick |
| `MAX_UPLOAD_SIZE_MB` | `5` | Proof-file upload cap |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://jaeger:4318` | Jaeger OTLP receiver |
| `OTEL_EXPORTER_OTLP_TLS_INSECURE` | `true` | TLS toggle for OTLP exporter |

## Graceful shutdown

`gracefulShutdown` in `cmd/api/main.go`:

1. Cancels the root context — the OrganicCanceler drains its current cycle.
2. Waits on `wg.Done()` (worker goroutine signals completion).
3. Calls `e.Shutdown(shutdownCtx)` — Echo drains in-flight HTTP requests.
4. Stops Prometheus metrics server.
5. Shuts down the OTel tracer provider (flushes pending spans).
6. Closes the DB connection pool.

This ordering ensures no goroutine accesses the DB after it is closed.

## Domain interfaces and sentinel errors

`internal/domain/` defines:

- Entity structs: `Household`, `WastePickup`, `Payment`.
- Service interfaces: `HouseholdService`, `PickupService`, `PaymentService`, `ReportService`.
- Repository interfaces: `HouseholdRepository`, `PickupRepository`, `PaymentRepository`.
- Infrastructure interfaces: `StorageService` (Upload, Delete, Ping).
- Sentinel errors (`internal/domain/errors.go`): `ErrNotFound`, `ErrConflict`,
  `ErrBusinessRule`, `ErrValidation`, `ErrRateLimit`, `ErrInternalFailure`.

Handlers map sentinel errors to HTTP status codes in
`internal/handler/handler.go:httpStatus`.

## Verification

- `go build ./...` — confirms DI wiring compiles.
- `go test ./internal/config/...` — config defaults and env-override tests.
- `go test ./internal/handler/...` — handler suite exercises graceful-error mapping.
