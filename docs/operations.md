# Operations

Runbook for diagnosing failures in the Community Waste Collection API.
See also the "Failure Modes" and "Troubleshooting" sections in `README.md`.

---

## Failure Mode Decision Tree

```mermaid
flowchart TD
    Start["Service reports an error or health check fails"] --> Q1{/readyz returns 200?}

    Q1 -- No --> Q2{Can connect to PostgreSQL?}
    Q2 -- No --> Fix1["Restart postgres container<br/>docker compose up postgres<br/>Check POSTGRES_DB, POSTGRES_USER, POSTGRES_PASSWORD"]
    Q2 -- Yes --> Fix2["Check migration state<br/>make migrate-status<br/>Run make migrate-up if dirty"]

    Q1 -- Yes --> Q3{Seeing 5xx responses?}

    Q3 -- Yes --> Q4{Panic in handler?}
    Q4 -- Yes --> Fix3["Check app logs for 'panic'<br/>runWithRecover logs the stack trace<br/>Investigate the offending request body"]
    Q4 -- No --> Q5{MinIO storage unreachable?}
    Q5 -- Yes --> Fix4["Restart minio container<br/>Check S3_ENDPOINT and S3_BUCKET_NAME<br/>Run mc admin info to verify bucket exists"]
    Q5 -- No --> Fix5["Check slog JSON output for<br/>trace_id + error fields<br/>Correlate trace_id in Jaeger UI"]

    Q3 -- No --> Q6{Seeing 429 on POST /api/pickups?}
    Q6 -- Yes --> Fix6["Rate limit hit — expected behavior<br/>Adjust RATE_LIMIT_RPS and RATE_LIMIT_BURST<br/>or wait for token bucket to refill"]

    Q6 -- No --> Q7{Worker not canceling organic pickups?}
    Q7 -- Yes --> Fix7["Check WORKER_CANCEL_INTERVAL and<br/>WORKER_ORGANIC_CUTOFF_DAYS env vars<br/>Check worker metric: pickup_cancellations_total"]

    Q7 -- No --> Q8{Grafana panels showing no data?}
    Q8 -- Yes --> Fix8["Check Promtail is running<br/>Curl http://localhost:3100/ready<br/>Verify Grafana datasource URLs<br/>Check prometheus scrape interval"]
```

---

## Health Endpoints

| Endpoint | Purpose | Failure means |
|---|---|---|
| `GET /health` | Liveness — server is up | Pod/container should be restarted |
| `GET /readyz` | Readiness — DB ping succeeds | Remove from load balancer; wait for DB |

---

## Log Correlation Cheat Sheet

Every application log line is JSON with `trace_id` and `span_id` fields:

```bash
# Find all log lines for a specific trace
docker compose -f deployments/docker-compose.yml logs app \
  | grep '"trace_id":"<your-trace-id>"'

# Query Loki directly
curl -sG 'http://localhost:3100/loki/api/v1/query_range' \
  --data-urlencode 'query={service="waste-api"} | json | trace_id = "<id>"' \
  --data-urlencode "start=$(date -d '1 hour ago' +%s)000000000" \
  --data-urlencode "end=$(date +%s)000000000"
```

---

## Common Recovery Commands

```bash
# Hard reset: stop all, wipe volumes, restart fresh
docker compose -f deployments/docker-compose.yml down -v --remove-orphans
make up && make migrate-up

# Re-create MinIO bucket after volume wipe
docker compose -f deployments/docker-compose.yml exec minio \
  mc alias set local http://localhost:9000 minioadmin minioadmin
docker compose -f deployments/docker-compose.yml exec minio \
  mc mb local/waste-collection-proofs

# Check migration state
docker run --rm \
  --network "$(docker compose -f deployments/docker-compose.yml ps -q postgres | xargs docker inspect --format '{{range .NetworkSettings.Networks}}{{.NetworkID}}{{end}}' | head -1)" \
  -v "$(pwd)/migrations:/migrations" \
  migrate/migrate:v4.18.1 \
  -path=/migrations -database "$DATABASE_URL" version
```
