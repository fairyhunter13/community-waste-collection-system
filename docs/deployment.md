# Deployment

Docker Compose stack, observability data paths, and graceful shutdown
sequence for the Community Waste Collection API.

---

## Docker Compose Topology

The full stack is defined in `deployments/docker-compose.yml`. A single
command starts all services: `make up` (or `docker compose -f deployments/docker-compose.yml up --build -d`).

```mermaid
graph LR
    subgraph App
        APP["app\n:8080 HTTP\n:2112 metrics"]
    end

    subgraph Storage
        PG["postgres:17\n:5432"]
        MN["minio\n:9000 S3 API\n:9001 console"]
    end

    subgraph Observability
        JG["jaeger\n:4318 OTLP HTTP\n:16686 UI"]
        LK["loki\n:3100 push/query"]
        PT["promtail\n(tail container stdout)"]
        GF["grafana\n:3000 UI"]
    end

    APP -->|SQL via sqlx| PG
    APP -->|S3 PutObject| MN
    APP -->|OTLP traces| JG
    APP -->|stdout JSON logs| PT
    PT -->|push log lines| LK
    GF -->|scrape /metrics| APP
    GF -->|query logs| LK
    GF -->|query traces| JG
```

**Named volumes:** `postgres_data`, `minio_data`, `loki_data`, `grafana_data`
— all persist across container restarts.

---

## Observability Data Flow

Three correlated signal types. All carry the same `trace_id`.

```mermaid
graph LR
    APP["app process"]

    APP -->|Prometheus scrape\nGET :2112/metrics| PROM["Grafana\n(Prometheus data source)"]
    APP -->|OTLP/HTTP POST\njaeger:4318| JAEGER["Jaeger UI\n:16686"]
    APP -->|slog JSON\nstdout| PT["Promtail"]
    PT -->|Loki push API\nloki:3100/loki/api/v1/push| LOKI["Loki"]
    LOKI -->|LogQL query| GF["Grafana\n:3000"]
    JAEGER -->|Trace link| GF
    PROM --> GF

    GF -->|"waste-collection" dashboard| D1["Request rates, latency,\npickup counts by type/status"]
    GF -->|"business-operations" dashboard| D2["Payment totals, BR violation rates,\nworker cancel counts"]
    GF -->|"logs-and-traces" dashboard| D3["Log stream with\ntrace_id deep-links to Jaeger"]
```

---

## Graceful Shutdown Sequence

The application catches SIGINT or SIGTERM and drains all in-flight work
before exiting. No request or background job is dropped on a clean
shutdown.

```mermaid
sequenceDiagram
    autonumber
    participant OS as OS / Docker
    participant M as main.go
    participant W as BR-04 Worker
    participant E as Echo HTTP server
    participant MS as Metrics server

    OS->>M: SIGTERM (or SIGINT)
    M->>M: signal.Notify fires\ncancel root context
    M->>W: context.Done() channel closes
    W->>W: ticker.Stop()\ndrain current tick if in-flight
    W-->>M: goroutine exits, wg.Done()
    M->>E: e.Shutdown(ctx) with HTTPShutdownTimeout
    E->>E: stop accepting new connections\nwait for in-flight handlers to complete
    E-->>M: shutdown complete
    M->>MS: metricsSrv.Shutdown(ctx)
    MS-->>M: metrics server stopped
    M->>M: tracerShutdown(ctx)\nflush pending OTel spans
    M->>M: wg.Wait() unblocks\nprocess exits 0
```

**Code:** `cmd/api/main.go:160-210`. The `sync.WaitGroup` ensures the
process does not exit until both the HTTP server and the worker have
finished draining.

---

## Running the Stack

```bash
# Start everything (build + detach)
make up

# Apply database migrations
make migrate-up

# Tail application logs
make logs

# Stop and remove containers + volumes
make down
```

See `README.md` → Quick Start for the full setup walkthrough including
MinIO bucket creation and Grafana login.
