# Phase 2 — Infrastructure & DevOps

## Purpose

Define all infrastructure, tooling configuration, environment setup, and developer workflow before writing application code. A developer following this plan should be able to run the complete stack with a single command on a clean machine.

---

## 1. Dockerfile (`build/Dockerfile`)

Multi-stage build: compile in a full Go image, run in a minimal Alpine image.

```dockerfile
# ─── Stage 1: Build ───────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install git for go mod download of VCS dependencies
RUN apk add --no-cache git ca-certificates

# Download dependencies first (cached layer if go.mod/go.sum unchanged)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
    -o /app/bin/api \
    ./cmd/api

# ─── Stage 2: Runtime ─────────────────────────────────────────────────────────
FROM alpine:3.22

WORKDIR /app

# ca-certificates: needed for TLS (S3, OTel); tzdata: correct timestamps
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/bin/api ./api

# API port | pprof debug port | Prometheus metrics port
EXPOSE 8080 6060 2112

ENTRYPOINT ["./api"]
```

**Build flags explained:**
- `CGO_ENABLED=0` — pure Go binary; works in scratch/alpine without glibc
- `-ldflags="-w -s"` — strip DWARF debug info and symbol table (~30% smaller binary)
- `-X main.version=...` — embed git tag as build metadata

---

## 2. Docker Compose (`deployments/docker-compose.yml`)

Four services: the application, PostgreSQL, MinIO, and OTel Collector.

```yaml
version: "3.9"

services:

  app:
    build:
      context: ..
      dockerfile: build/Dockerfile
    ports:
      - "8080:8080"    # API
      - "6060:6060"    # pprof (internal — do not expose in production)
      - "2112:2112"    # Prometheus metrics (internal)
    env_file: ../.env
    depends_on:
      postgres:
        condition: service_healthy
      minio:
        condition: service_healthy
    restart: unless-stopped

  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: waste_collection
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d waste_collection"]
      interval: 5s
      timeout: 5s
      retries: 10
      start_period: 10s

  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    ports:
      - "9000:9000"    # S3-compatible API
      - "9001:9001"    # MinIO console
    volumes:
      - minio_data:/data
    healthcheck:
      test: ["CMD", "mc", "ready", "local"]
      interval: 5s
      timeout: 5s
      retries: 10
      start_period: 10s

  otel-collector:
    image: otel/opentelemetry-collector-contrib:latest
    command: ["--config=/etc/otel-collector-config.yaml"]
    volumes:
      - ./otel-collector-config.yaml:/etc/otel-collector-config.yaml:ro
    ports:
      - "4317:4317"    # OTLP gRPC receiver
      - "4318:4318"    # OTLP HTTP receiver
      - "8888:8888"    # Collector's own metrics

volumes:
  postgres_data:
  minio_data:
```

**OTel Collector config (`deployments/otel-collector-config.yaml`):**
```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:

exporters:
  logging:
    loglevel: debug   # Log traces to stdout for local dev visibility

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [logging]
```

---

## 3. PostgreSQL Schema (migrations/)

### Migration naming convention
```
{sequence:06d}_{description}.{up|down}.sql
```
Example: `000001_create_households.up.sql`

All sequences must apply cleanly in order and revert cleanly in reverse order.

---

### `migrations/000001_create_households.up.sql`
```sql
CREATE TABLE IF NOT EXISTS households (
    id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_name  TEXT          NOT NULL CHECK (char_length(owner_name) >= 1),
    address     TEXT          NOT NULL CHECK (char_length(address) >= 1),
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);
```

### `migrations/000001_create_households.down.sql`
```sql
DROP TABLE IF EXISTS households;
```

---

### `migrations/000002_create_pickups.up.sql`
```sql
CREATE TYPE waste_type AS ENUM ('organic', 'plastic', 'paper', 'electronic');
CREATE TYPE pickup_status AS ENUM ('pending', 'scheduled', 'completed', 'canceled');

CREATE TABLE IF NOT EXISTS waste_pickups (
    id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID          NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    type         waste_type    NOT NULL,
    status       pickup_status NOT NULL DEFAULT 'pending',
    pickup_date  TIMESTAMPTZ,
    safety_check BOOLEAN       NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- General query indexes
CREATE INDEX idx_pickups_household_id  ON waste_pickups(household_id);
CREATE INDEX idx_pickups_status        ON waste_pickups(status);
CREATE INDEX idx_pickups_type_status   ON waste_pickups(type, status);

-- Partial index for the organic auto-cancel worker query (performance optimization)
-- Only indexes rows that the worker will ever query
CREATE INDEX idx_pickups_organic_pending_created
    ON waste_pickups(created_at)
    WHERE type = 'organic' AND status = 'pending';
```

### `migrations/000002_create_pickups.down.sql`
```sql
DROP INDEX IF EXISTS idx_pickups_organic_pending_created;
DROP INDEX IF EXISTS idx_pickups_type_status;
DROP INDEX IF EXISTS idx_pickups_status;
DROP INDEX IF EXISTS idx_pickups_household_id;
DROP TABLE IF EXISTS waste_pickups;
DROP TYPE IF EXISTS pickup_status;
DROP TYPE IF EXISTS waste_type;
```

---

### `migrations/000003_create_payments.up.sql`
```sql
CREATE TYPE payment_status AS ENUM ('pending', 'paid', 'failed');

CREATE TABLE IF NOT EXISTS payments (
    id             UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id   UUID           NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    waste_id       UUID           NOT NULL UNIQUE REFERENCES waste_pickups(id) ON DELETE CASCADE,
    amount         NUMERIC(12,2)  NOT NULL CHECK (amount > 0),
    payment_date   TIMESTAMPTZ,
    status         payment_status NOT NULL DEFAULT 'pending',
    proof_file_url TEXT,
    created_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- UNIQUE constraint on waste_id already creates an index — no explicit index needed
CREATE INDEX idx_payments_household_id  ON payments(household_id);
CREATE INDEX idx_payments_status        ON payments(status);
CREATE INDEX idx_payments_payment_date  ON payments(payment_date)
    WHERE payment_date IS NOT NULL;  -- Partial index; only paid payments have a date
```

### `migrations/000003_create_payments.down.sql`
```sql
DROP INDEX IF EXISTS idx_payments_payment_date;
DROP INDEX IF EXISTS idx_payments_status;
DROP INDEX IF EXISTS idx_payments_household_id;
DROP TABLE IF EXISTS payments;
DROP TYPE IF EXISTS payment_status;
```

---

## 4. golangci-lint Configuration (`.golangci.yml`)

```yaml
run:
  timeout: 5m
  go: "1.26"
  modules-download-mode: readonly

linters:
  enable:
    # ── Correctness ─────────────────────────────────────────────────────────
    - errcheck         # all error return values must be checked
    - govet            # go vet suite: shadow, printf, structtag, etc.
    - staticcheck      # 150+ checks: SA (bugs), ST (style), QF (quickfixes), S1 (simplifications)
    - gosec            # security: SQL injection, path traversal, hardcoded secrets, weak crypto
    - bodyclose        # HTTP response bodies must be closed
    - rowserrcheck     # sql.Rows.Err() must be checked after iteration
    - sqlclosecheck    # sql.Rows and sql.Stmt must be closed
    - noctx            # HTTP client calls must use a context-aware method
    - contextcheck     # context must be passed properly through call chains
    - exhaustive       # switch statements on enums must be exhaustive

    # ── Code quality ────────────────────────────────────────────────────────
    - revive           # opinionated Go style linter (successor to golint)
    - unused           # unused exported and unexported code
    - ineffassign      # assignments where the value is never used
    - goconst          # repeated string literals should be named constants
    - godot            # exported comments must end with a period
    - misspell         # common English spelling mistakes in code and comments
    - cyclop           # cyclomatic complexity (max 15 per function)
    - wrapcheck        # errors returned from external packages must be wrapped
    - prealloc         # slice append loops where pre-allocation is possible

    # ── Style and clarity ───────────────────────────────────────────────────
    - goimports        # gofmt + correct import grouping (stdlib / external / internal)
    - whitespace       # unnecessary blank lines at start/end of blocks
    - godox            # flag TODO/FIXME/HACK/BUG comments (warn, not fail)

linters-settings:
  cyclop:
    max-complexity: 15
    skip-tests: true

  gosec:
    excludes:
      - G115  # integer conversion overflow — too noisy for general use

  revive:
    rules:
      - name: exported
        arguments: ["checkPrivateReceivers"]
      - name: var-naming
      - name: package-comments
      - name: unexported-return

  wrapcheck:
    ignorePackageGlobs:
      # Domain errors are the canonical errors; no wrapping needed when returning them
      - "github.com/fairyhunter13/community-waste-collection-system/internal/domain"

  goimports:
    # Treat internal packages as a separate group from external dependencies
    local-prefixes: "github.com/fairyhunter13/community-waste-collection-system"

  goconst:
    min-len: 3
    min-occurrences: 3

  godox:
    keywords:
      - TODO
      - FIXME
      - HACK
      - BUG

issues:
  max-same-issues: 0
  exclude-rules:
    # Relax security and wrapping checks in test files
    - path: "_test\\.go"
      linters: [gosec, wrapcheck, cyclop]
    # Allow init functions in main package
    - path: "cmd/"
      linters: [gochecknoinits]
```

---

## 5. Complete Makefile

```makefile
# ── Variables ─────────────────────────────────────────────────────────────────
BINARY     := api
CMD        := ./cmd/api
MIGRATIONS := migrations
MODULE     := github.com/fairyhunter13/community-waste-collection-system
DB_URL     ?= $(DATABASE_URL)

.PHONY: all run build clean \
        lint fmt vet \
        test test-unit test-integration test-e2e bench perf \
        migrate-up migrate-down migrate-force migrate-version migrate-create \
        docker-up docker-down docker-logs docker-clean \
        seed

# ── Default ───────────────────────────────────────────────────────────────────
all: lint test build

# ── Development ───────────────────────────────────────────────────────────────
run:
	go run $(CMD)

build:
	CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/$(BINARY) $(CMD)

clean:
	rm -rf bin/ coverage.out coverage.html

# ── Code Quality ──────────────────────────────────────────────────────────────
lint:
	golangci-lint run ./...

fmt:
	goimports -w -local $(MODULE) .

vet:
	go vet ./...

# ── Testing ───────────────────────────────────────────────────────────────────
test: test-unit

test-unit:
	go test -race -count=1 -coverprofile=coverage.out \
	    -coverpkg=./internal/... \
	    ./internal/... -v

test-integration:
	go test -race -count=1 -tags=integration \
	    ./internal/repository/... ./internal/service/... \
	    -timeout 120s -v

test-e2e:
	go test -race -count=1 -tags=e2e \
	    ./test/e2e/... \
	    -timeout 180s -v

bench:
	go test -bench=. -benchmem -run='^$$' -tags=integration \
	    ./internal/... \
	    -timeout 120s

perf:
	go test -bench=. -benchmem -run='^$$' -tags=perf \
	    ./test/perf/... \
	    -timeout 300s

coverage:
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ── Migrations ────────────────────────────────────────────────────────────────
migrate-up:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" up

migrate-down:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" down

migrate-force:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" force $(VERSION)

migrate-version:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" version

migrate-create:
	migrate create -ext sql -dir $(MIGRATIONS) -seq $(NAME)

# ── Docker ────────────────────────────────────────────────────────────────────
docker-up:
	docker compose -f deployments/docker-compose.yml up --build -d

docker-down:
	docker compose -f deployments/docker-compose.yml down

docker-logs:
	docker compose -f deployments/docker-compose.yml logs -f app

docker-clean:
	docker compose -f deployments/docker-compose.yml down -v --remove-orphans

# ── Seed ──────────────────────────────────────────────────────────────────────
seed:
	psql "$(DB_URL)" -f scripts/seed.sql
```

**Usage examples:**
```bash
make docker-up          # start all services
make migrate-up         # apply all migrations
make run                # start app outside Docker
make test               # unit tests
make test-integration   # integration tests (requires DB)
make test-e2e           # E2E tests (requires full docker-compose stack)
make bench              # run DB-layer benchmarks (requires DATABASE_URL)
make perf               # run HTTP performance tests (requires BASE_URL + full stack)
make lint               # full golangci-lint
make migrate-create NAME=add_index_foo  # create new migration pair
```

---

## 6. Environment Variables (`.env.example`)

```env
# ── Server ────────────────────────────────────────────────────────────────────
APP_PORT=8080
APP_ENV=development      # development | production

# ── Debug (pprof — NEVER expose publicly in production) ──────────────────────
DEBUG_PORT=6060

# ── Database ──────────────────────────────────────────────────────────────────
DATABASE_URL=postgres://postgres:postgres@localhost:5432/waste_collection?sslmode=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_IDLE_TIME=5m

# ── S3-compatible Storage ─────────────────────────────────────────────────────
S3_ENDPOINT=http://localhost:9000
S3_BUCKET=waste-proofs
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=minioadmin
S3_REGION=us-east-1
S3_USE_PATH_STYLE=true   # Required for MinIO; false for AWS S3

# ── File Upload Limits ────────────────────────────────────────────────────────
MAX_UPLOAD_SIZE_MB=10

# ── Rate Limiting ─────────────────────────────────────────────────────────────
RATE_LIMIT_RPS=5         # Requests per second per IP (pickup creation)
RATE_LIMIT_BURST=10      # Burst capacity above the sustained rate

# ── Observability ─────────────────────────────────────────────────────────────
LOG_LEVEL=info           # debug | info | warn | error
LOG_FORMAT=json          # json | text
METRICS_PORT=2112        # Prometheus /metrics endpoint
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
OTEL_SERVICE_NAME=community-waste-collection-api
OTEL_SERVICE_VERSION=0.1.0

# ── Background Worker ─────────────────────────────────────────────────────────
WORKER_CANCEL_INTERVAL=1h    # How often the organic canceler runs
WORKER_ORGANIC_CUTOFF_DAYS=3 # Days before an organic pickup is auto-canceled
```

---

## 7. Pre-commit Git Hook (`.githooks/pre-commit`)

```bash
#!/bin/sh
set -e

echo "Running pre-commit checks..."

go build ./...
go vet ./...
golangci-lint run ./...
go test -race -count=1 ./internal/... -short

echo "All checks passed."
```

**Install:**
```bash
git config core.hooksPath .githooks
chmod +x .githooks/pre-commit
```

---

## 8. Local Development First-Run Workflow

```bash
# Step 1: Clone and configure environment
cp .env.example .env
# Edit .env if needed (defaults work for local docker-compose)

# Step 2: Start all infrastructure services
make docker-up
# Waits for postgres and minio health checks to pass

# Step 3: Apply database migrations
make migrate-up
# Should print: 3/u create_households, 3/u create_pickups, 3/u create_payments

# Step 4: Optional — seed sample data
make seed

# Step 5: Run the application locally (outside Docker for fast iteration)
make run
# Server starts at :8080, debug at :6060, metrics at :2112

# Step 6: Verify all endpoints
curl http://localhost:8080/api/households
curl http://localhost:6060/debug/pprof/
curl http://localhost:2112/metrics

# Step 7: Run tests
make test
make test-integration

# Step 8: Check code quality
make lint
```

---

## 9. Infrastructure Verification Checklist

Before proceeding to Phase 3 (Development), verify:

- [ ] `docker build -f build/Dockerfile .` completes without error
- [ ] `make docker-up` starts all 4 services, all show `healthy`
- [ ] `make migrate-up` applies all 3 migrations successfully
- [ ] `make migrate-down` reverts all migrations cleanly (then re-apply)
- [ ] `make migrate-version` shows current migration version
- [ ] `golangci-lint run ./...` passes on an empty Go package
- [ ] `.env.example` contains every variable the application reads
- [ ] `make docker-down` stops all services without error
