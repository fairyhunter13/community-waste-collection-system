# Contributing

Short, pragmatic onboarding for the most common tasks. See
[`README.md`](README.md) for project overview and `docs/adr/` for the design
record behind the choices below.

## Prerequisites

- Go 1.26+
- Docker + Docker Compose
- `make`, `psql`, `migrate` CLI (`go install -tags postgres
  github.com/golang-migrate/migrate/v4/cmd/migrate@latest`)

## One-line bootstrap

```bash
make docker-up           # boots Postgres, MinIO, OTel, Jaeger, Prometheus, Grafana, app
curl -fsS http://localhost:8080/health  # should print {"status":"ok"}
```

## Hot-reload dev loop

For rapid iteration without a full docker-compose stack, use [air](https://github.com/air-verse/air):

```bash
go install github.com/air-verse/air@latest
make dev   # rebuilds and restarts the binary on every .go file change
```

Requires a local Postgres (`DATABASE_URL` env var) and MinIO instance, or
set `STORAGE_MOCK=true` to skip S3 uploads during local development.

## Running tests

```bash
make test-unit            # unit only, with -race
make test-integration     # testcontainers Postgres
make test-e2e             # full docker-compose stack
make test                 # all of the above

go test -run TestPickupService_Create_BR01 -race ./internal/service/...
```

## Coverage

```bash
make test-coverage        # writes coverage.out, prints function summary
make coverage-gate        # enforces the 80% threshold from CI logic
```

## Lint

```bash
make lint                 # golangci-lint v2.12.2
make fmt                  # gofmt + goimports
```

## Regenerating mocks

Service-level mocks are mockery-generated:

```bash
go generate ./...
```

Each `internal/domain/<service>.go` carries a `//go:generate` directive at the
top.

## Adding an endpoint

1. **Domain**: define request/response in `internal/domain/<resource>.go`.
2. **Service**: implement the business rule + tracing/metrics in
   `internal/service/<resource>.go`. Wrap mutations that touch BR-01/05
   in a transaction (`internal/repository/*` exposes `*sqlx.DB` for
   `db.BeginTxx`).
3. **Repository**: SQL lives in `internal/repository/<resource>.go`. Conditional
   UPDATEs (`WHERE status = expected`) protect against TOCTOU races.
4. **Handler**: Echo binding + validation lives in
   `internal/handler/<resource>.go`. Use `respondError(c, status, code, msg)`
   so the envelope matches the documented contract.
5. **Routes**: register in `internal/handler/handler.go`. Apply BodyLimit on
   any JSON POST/PUT and rate limiting on any expensive write.
6. **OpenAPI**: add the path + schemas to `api/openapi.yaml`.
7. **Postman**: add a request to `api/community-waste.postman_collection.json`.
8. **Tests**: at minimum a handler-level unit test asserting the happy path
   and the validation error envelope, plus an E2E for the new business path.

## Adding a business rule

Business rules live in the **service** layer (see
[`docs/adr/0006-business-rules-in-service-layer.md`](docs/adr/0006-business-rules-in-service-layer.md)),
not in handlers and not in repositories. The repository surfaces
`pq.Error` codes; the service translates to `domain.ErrConflict` /
`ErrValidation` / `ErrNotFound`; the handler turns sentinel errors into
HTTP status codes via `mapDomainError`.

When the rule is concurrency-sensitive, add both an application-tier guard
(advisory lock or `SELECT … FOR UPDATE`) **and** a schema-tier guard (partial
unique index or `CHECK`). Two locks beat one race.

## Running a single test

```bash
go test -run TestPickupHandler/TestCreatePickup_201 -race ./internal/handler/
go test -run TestE2E_ConcurrentCompletes_OnlyOneSucceeds -race -count=1 -v \
  ./test/e2e/
```

## Architecture decisions

Before introducing a pattern that's not already in the codebase, check
`docs/adr/` — eight numbered records cover the load-bearing choices
(no-ORM, sentinel errors, shopspring/decimal, per-IP rate limit, worker
context cancellation, business rules in service layer, OTel, Prometheus
RED). New decisions belong as a new `docs/adr/0009-….md`.
