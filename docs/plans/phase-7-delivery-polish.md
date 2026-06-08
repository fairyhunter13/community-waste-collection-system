# Phase 7 — Delivery Polish

Covers local developer experience, CI pipeline, API contract documentation,
and Docker Compose stack.

## Docker Compose stack

`deployments/docker-compose.yml` defines:

| Service | Image | Exposes |
|---------|-------|---------|
| `db` | postgres:17-alpine | 5432 (internal) |
| `minio` | minio/minio | 9000/9001 (console) |
| `createbuckets` | minio/mc | init job — creates waste-proofs bucket |
| `app` | built from Dockerfile | 8080 (HTTP), 2112 (Prometheus) |
| `prometheus` | prom/prometheus | 9090 |
| `grafana` | grafana/grafana | 3000 |
| `jaeger` | jaegertracing/all-in-one | 16686 (UI), 4318 (OTLP) |

`docker compose up --build` starts the full stack. The app binary waits for
the DB and MinIO to be healthy before starting (via `depends_on: condition: service_healthy`).

pprof debug server binds to `127.0.0.1:6060` — not exposed as a host port.

## Makefile targets

Key targets (run `make help` for the full list):

| Target | Purpose |
|--------|---------|
| `make run` | `docker compose up --build` |
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Roll back the latest migration |
| `make seed` | Run `scripts/seed.sql` to insert sample data |
| `make test` | Unit tests |
| `make test-integration` | Integration tests |
| `make test-e2e` | End-to-end tests against docker compose stack |
| `make lint` | golangci-lint |
| `make coverage` | Generate HTML coverage + 80% gate |
| `make dashboards-lint` | Grafana dashboard JSON validation |
| `make load-test` | k6 load test |

## CI workflow

`.github/workflows/ci.yml` runs on every push and pull request.

Job graph:

```
lint ──────────────────────────────┐
build ─────────────────────────────┤
test ──────────────────────────────┤──► (all pass) ──► coverage
test-integration ──────────────────┤
test-e2e ──────────────────────────┤
vulnerability-scan ────────────────┤
api-contract (Redocly lint) ───────┤
dashboards-lint ───────────────────┘
```

- **lint**: golangci-lint v2 with `staticcheck`, `govet`, `errcheck`, `revive`.
- **vulnerability-scan**: Trivy (container image) + govulncheck (module).
- **api-contract**: Redocly CLI validates `api/openapi.yaml` against the OpenAPI 3.1 spec.
- **coverage**: Uploads to Codecov; gate at 80% line coverage.

## OpenAPI contract

`api/openapi.yaml` documents all 15 product endpoints with request/response schemas,
error shapes, and example payloads. Validated by Redocly in CI.

`api/community-waste.postman_collection.json` and
`api/community-waste.postman_environment.json` provide a ready-to-import Postman
collection for manual exploration.

## Version endpoint

`GET /version` returns build metadata (version string, git SHA, build time) injected
at link time via `-ldflags`.

## Verification

```bash
docker compose -f deployments/docker-compose.yml up --build -d
sleep 15
curl -fsS http://localhost:8080/health           # expect {"status":"ok"}
curl -fsS http://localhost:8080/readyz           # expect {"db":"ok","storage":"ok"}
curl -fsS http://localhost:8080/metrics | head   # expect Prometheus text format
curl -fsS http://localhost:8080/version          # expect build metadata
make lint
make test
make test-integration
make test-e2e
make dashboards-lint
docker compose -f deployments/docker-compose.yml down
```
