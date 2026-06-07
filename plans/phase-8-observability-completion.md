# Phase 8 — Observability Completion

## Purpose

Extend the observability stack from "traces + scrape-based metrics + Promtail
tails" to full OTLP-native signal delivery across all three pillars — traces,
metrics, AND logs — while keeping every existing path live. Grafana, Prometheus,
and Loki all keep working exactly as they do today; the OTLP paths are
**additive**. A reviewer can switch off the OTLP collector entirely and the
stack degrades gracefully rather than falling over.

This file also covers Tier 13 (error visibility & source attribution): a
reviewer auditing any 4xx or 5xx should be able to pivot from the HTTP response
to a Loki query to a Jaeger trace in under thirty seconds, and every log line
should carry enough context to answer "which file emitted this?" without opening
the source.

The requirements PDF and `REQUIREMENTS_RAW.md` remain gitignored. No
identifying information from the requirements document is permitted in any
committed file.

---

## Tier 11 — OpenTelemetry Completion (T72–T78)

### Context

Traces already ship via OTLP → Jaeger (wired in T17/T18 of Phase 6).
`/metrics` is served via `promhttp.Handler` backed by a Prometheus registry,
and Promtail tails the container log stream into Loki. The goal here is to
wire OTLP push for metrics and logs **alongside** those paths, route all three
through the collector, and surface exemplar dots on the histogram heatmaps so
a reviewer can click straight through to a Jaeger trace.

### T72 — Dual-exporter MeterProvider

Add a shared `MeterProvider` that satisfies two sinks simultaneously:

1. A Prometheus reader that backs the existing `/metrics` endpoint — zero
   change to the Prometheus scrape path.
2. An OTLP push exporter (`go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp`)
   targeting the collector at `${OTEL_EXPORTER_OTLP_ENDPOINT}` (default
   `http://otel-collector:4318`).

Files:
- `internal/observability/metrics.go` — replace bare `prometheus.NewRegistry()`
  wiring with `metric.NewMeterProvider(metric.WithReader(prometheusReader),
  metric.WithReader(otlpReader))`. The `prometheus.Exporter` continues to
  expose `Handler()` for the `/metrics` route.
- `internal/observability/otel.go` — export a `NewMeterProvider(res *resource.Resource) *metric.MeterProvider` constructor so `main.go` can pass the shared resource (see T74).
- `cmd/api/main.go` — call `observability.NewMeterProvider(res)` and pass
  the returned provider to `otel.SetMeterProvider`. Shutdown hook:
  `defer mp.Shutdown(ctx)`.

Shape sketch:

```go
// internal/observability/otel.go
func NewMeterProvider(res *resource.Resource) (*metric.MeterProvider, *prometheus.Exporter, error) {
    promExp, _ := prometheus.New()
    otlpExp, _ := otlpmetrichttp.New(context.Background())
    mp := metric.NewMeterProvider(
        metric.WithResource(res),
        metric.WithReader(metric.NewPeriodicReader(otlpExp, metric.WithInterval(15*time.Second))),
        metric.WithReader(promExp),
    )
    return mp, promExp, nil
}
```

### T73 — OTLP log bridge via otelslog

Add `go.opentelemetry.io/contrib/bridges/otelslog` as a second `slog.Handler`
multiplexed alongside the existing stdout JSON handler. Log records flow to
both sinks:

- Existing path: JSON lines → stdout → Promtail → Loki (unchanged).
- New path: `otelslog.NewHandler` → OTel LoggerProvider → OTLP → collector →
  Loki exporter in collector (configured in T75).

Files:
- `internal/observability/logger.go` — wrap the two handlers in a
  `slog.NewLogger(slog.NewMultiHandler(jsonHandler, otelslogHandler))`.
  Export a `NewLoggerProvider` constructor that accepts the shared resource.
- `cmd/api/main.go` — construct `LoggerProvider`, register as
  `global.SetLoggerProvider(lp)`, pass to `observability.NewLogger`. Shutdown:
  `defer lp.Shutdown(ctx)`.

The existing stdout JSON handler is **not** removed. Promtail continues to
work. The `otelslog` bridge is a no-op if the collector is unreachable
(OTLP exporter uses a non-blocking retry queue).

Shape sketch:

```go
// internal/observability/logger.go
func NewLogger(lp log.LoggerProvider, addSource bool) *slog.Logger {
    jsonH  := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: addSource})
    otlpH  := otelslog.NewHandler("community-waste-collection-api", otelslog.WithLoggerProvider(lp))
    return slog.New(slogmulti.Fanout(jsonH, otlpH))
}
```

### T74 — Shared resource with `ServiceVersion`

All three providers (tracer, meter, logger) must share one
`*resource.Resource` so every signal carries identical `service.name`,
`service.version`, and `deployment.environment` attributes.

File: `internal/observability/otel.go`

```go
func NewResource(version string) (*resource.Resource, error) {
    return resource.Merge(
        resource.Default(),
        resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String("community-waste-collection-api"),
            semconv.ServiceVersionKey.String(version),
            semconv.DeploymentEnvironmentKey.String(os.Getenv("APP_ENV")),
        ),
    )
}
```

`version` is injected via the existing `-X main.version=$(git describe --tags)`
ldflags in `build/Dockerfile` and `Makefile`. `cmd/api/main.go` calls
`NewResource(version)` once and threads the result into `NewTracerProvider`,
`NewMeterProvider`, and `NewLoggerProvider`.

### T75 — Collector config for metrics and log ingestion

Extend `deployments/otel-collector-config.yaml` to:

1. **Receive** OTLP/HTTP on `:4318` — already present for traces; add metrics
   and logs to the same receiver's `protocols` block.
2. **Export metrics** via a `prometheus` exporter on `:8889` so the collector
   itself becomes a Prometheus scrape target (secondary; primary is still the
   app's `/metrics`).
3. **Export logs** via a `loki` exporter pointing at `http://loki:3100/loki/api/v1/push`.
   Label set: `service_name`, `deployment_environment`, `severity`.

Shape sketch:

```yaml
# deployments/otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

exporters:
  jaeger:
    endpoint: jaeger:14250
    tls: {insecure: true}
  prometheus:
    endpoint: "0.0.0.0:8889"
    namespace: otelcol
  loki:
    endpoint: http://loki:3100/loki/api/v1/push
    default_labels_enabled:
      exporter: false
      job: false
    labels:
      attributes:
        service.name: "service_name"
        deployment.environment: "env"
        level: "severity"

service:
  pipelines:
    traces:   {receivers: [otlp], exporters: [jaeger]}
    metrics:  {receivers: [otlp], exporters: [prometheus]}
    logs:     {receivers: [otlp], exporters: [loki]}
```

Add a `prometheus.yml` scrape job for the collector's `:8889` endpoint
alongside the existing app scrape target.

### T76 — Exemplars on request and DB histograms

Enable exemplar injection so histogram heatmap cells in Grafana carry a
clickable sample pointing to the Jaeger trace that produced that observation.

File: `internal/observability/metrics.go`

Use `metric.WithExemplarFilter(exemplar.AlwaysOnFilter)` when constructing the
`PeriodicReader` for the OTLP exporter, and ensure the Prometheus reader is
initialized with `prometheus.New(prometheus.WithoutUnits())` plus
`prometheus.WithExemplarFromContext` (available in
`go.opentelemetry.io/otel/exporters/prometheus` v0.52+).

The two histograms that need exemplars:
- `http_request_duration_seconds` (already defined in `internal/observability/metrics.go`)
- `db_query_duration_seconds` (already defined in `internal/observability/metrics.go`)

No changes to how those instruments are recorded — the exemplar is plumbed
automatically via the OTel context carrying the active span.

### T77 — Grafana histogram heatmap panel with exemplar dots

Add a **Histogram heatmap** panel to the existing API dashboard
(`deployments/grafana/dashboards/waste-collection.json` or `api.json`,
whichever exists):

- **Panel type**: `histogram` (Grafana 10 native heatmap) or `heatmap`
- **Query**:
  ```
  sum by (le) (rate(http_request_duration_seconds_bucket[5m]))
  ```
- **Exemplars**: enabled. Clicking a dot opens the Jaeger trace via the
  existing `jaeger` data source. Set the exemplar label `TraceID` → Jaeger
  traceId URL (data source field `traceIdLabelName`).
- Place in the "Latency" row between the existing p95 stat panel and the
  current histogram panel.

The JSON patch is small: add one panel object with `type: "heatmap"`,
`options.exemplars.enabled: true`, and the Jaeger data-source reference. No
existing panels are removed.

### T78 — Trace the MinIO/S3 HTTP client

When `PUT /api/payments/:id/confirm` uploads a proof file, the S3 round-trip
shows in Jaeger as a single opaque span (or not at all). Wrap the `http.Client`
used by the AWS SDK (or MinIO client) with `otelhttp.NewTransport` so the HTTP
sub-spans appear as children of the handler span.

File: `internal/storage/s3.go` (or wherever the MinIO/S3 client is
constructed — confirm via grep if the file name differs).

```go
// internal/storage/s3.go
import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

httpClient := &http.Client{
    Transport: otelhttp.NewTransport(http.DefaultTransport),
}
// pass httpClient into minio.New(..., &minio.Options{Transport: httpClient.Transport})
// or into the AWS config httpclient option
```

Add a `TestStorage_S3Client_UsesOtelTransport` unit test in
`internal/storage/s3_test.go` (or a new file) that asserts the client's
`Transport` is not bare `http.DefaultTransport`.

---

## Tier 13 — Error Visibility & Source Attribution (T83–T88)

### Context

When a reviewer is auditing a failing request they currently must: read the
response body (no trace_id), switch to Grafana, search Loki by approximate
timestamp, hope the log line appeared in the same second, then guess which
source file emitted it. The goal is to collapse that workflow to: read the
error response (trace_id is there), paste into the Loki textbox filter,
click the trace link.

This tier is additive to Tier 11 — it relies on the shared resource and the
dual-exporter logger constructed above.

### T83 — `AddSource: true` in `observability.NewLogger`

The simplest change: set `slog.HandlerOptions{AddSource: true}` in the JSON
handler options inside `observability.NewLogger`.

File: `internal/observability/logger.go`

```go
jsonH := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    AddSource:   true,
    ReplaceAttr: levelToString,
})
```

Every log line emitted anywhere in the call stack now carries:

```json
{"source":{"function":"github.com/…/service.(*PickupService).Create","file":"internal/service/pickup.go","line":47}, …}
```

The `otelslog` bridge propagates source info as OTel log record attributes
`code.filepath`, `code.function`, `code.lineno` (semconv).

### T84 — `meta: {request_id, trace_id, span_id}` in error envelope

The standard error envelope (`internal/handler/handler.go`) currently returns:

```json
{"success": false, "error": {"code": "…", "message": "…"}}
```

Extend to:

```json
{
  "success": false,
  "error": {"code": "…", "message": "…"},
  "meta": {"request_id": "…", "trace_id": "…", "span_id": "…"}
}
```

File: `internal/handler/handler.go`

```go
type errorMeta struct {
    RequestID string `json:"request_id,omitempty"`
    TraceID   string `json:"trace_id,omitempty"`
    SpanID    string `json:"span_id,omitempty"`
}

type errorEnvelope struct {
    Success bool       `json:"success"`
    Error   apiError   `json:"error"`
    Meta    *errorMeta `json:"meta,omitempty"`
}

func respondError(c echo.Context, status int, code, msg string) error {
    env := errorEnvelope{Success: false, Error: apiError{Code: code, Message: msg}}
    sc  := trace.SpanFromContext(c.Request().Context()).SpanContext()
    if sc.IsValid() {
        env.Meta = &errorMeta{
            RequestID: c.Request().Header.Get("X-Request-ID"),
            TraceID:   sc.TraceID().String(),
            SpanID:    sc.SpanID().String(),
        }
    }
    return c.JSON(status, env)
}
```

`meta` is omitted when there is no active span (e.g. unit tests without OTel),
so existing tests that assert the error shape are unaffected unless they
explicitly check for `meta`. Update `internal/handler/error_envelope_test.go`
to assert `meta.trace_id` is a 32-hex-character string when a span is active.

### T85 — Promtail pipeline: extract source fields

Promtail's pipeline stage in `deployments/promtail-config.yaml` currently
extracts `level`, `trace_id`, `span_id`, and `request_id`. Extend it to also
extract the nested `source` fields that `slog` now emits:

```yaml
# deployments/promtail-config.yaml (pipeline_stages section)
- json:
    expressions:
      level:           level
      trace_id:        trace_id
      span_id:         span_id
      request_id:      request_id
      op:              op
      source_file:     source.file
      source_function: source.function
      source_line:     source.line

- labels:
    level:
    trace_id:
    span_id:
    source_file:
    source_function:
```

`source_file` and `source_function` become Loki labels so the Grafana log
panel can filter by them directly without a full-text search.

### T86 — Graduated severity in service methods

Audit all four service files and align log levels with the following policy:

| Situation | Level |
|---|---|
| Entry/exit of every public method (debug builds) | `DEBUG` |
| Successful state transition (pickup scheduled, payment confirmed) | `INFO` |
| Domain-constraint violation returned to caller (ErrConflict, ErrValidation, ErrNotFound) | `WARN` |
| Unexpected storage or infrastructure error | `ERROR` |

Files: `internal/service/pickup.go`, `internal/service/payment.go`,
`internal/service/report.go`, `internal/service/household.go`

Key changes:
- Downgrade `ErrConflict` / `ErrValidation` returns from `ERROR` (or silent)
  to `WARN` — these are expected domain outcomes, not bugs.
- Ensure every `ERROR` log includes `"err", err` as a structured attribute.
- Ensure every state-transition `INFO` log includes `"op"` (the operation
  name, e.g. `"pickup.schedule"`) so the Promtail `op` label is populated.

### T87 — Grafana logs-and-traces dashboard extensions

Extend `deployments/grafana/dashboards/logs-and-traces.json` with:

1. **Two textbox variables** at dashboard level:
   - `request_id` — label filter `{request_id="$request_id"}` in Loki panels.
   - `source_file` — label filter `{source_file=~"$source_file"}`.

2. **"Error correlation" row** (collapsed by default) containing:
   - A **Logs panel** (Loki datasource) filtered by `{level=~"error|warn"}`
     with columns: `timestamp`, `source_function`, `trace_id`, `request_id`,
     `msg`. Clicking `trace_id` navigates to Jaeger (derived field, already
     wired from T19 in Phase 6 — reuse the same `tracesToLogsV2` link).
   - A **Table panel** (Loki datasource + `sum by (source_function)`) showing
     error count by source function over the selected time window.

3. **Variable linking**: the existing trace panel's `trace_id` variable drives
   the Loki `request_id` search via a dashboard link (not a cross-datasource
   join — just a URL query-param link to avoid Grafana licensing features).

No existing panels are removed.

### T88 — E2E error-visibility proof test

New file: `test/e2e/error_visibility_test.go` (build tag `//go:build e2e`)

```go
func TestErrorVisibility_FullChain(t *testing.T) {
    // 1. POST invalid payload → expect 422
    resp := postInvalidPickup(t)
    require.Equal(t, 422, resp.StatusCode)

    var env errorEnvelope
    json.NewDecoder(resp.Body).Decode(&env)
    reqID   := resp.Header.Get("X-Request-Id")
    traceID := env.Meta.TraceID
    spanID  := env.Meta.SpanID
    require.NotEmpty(t, reqID)
    require.Len(t, traceID, 32)  // 128-bit hex, no dashes
    require.Len(t, spanID,  16)

    // 2. Query Loki for logs carrying this request_id
    //    (via the Loki HTTP API, port 3100)
    lines := queryLoki(t, fmt.Sprintf(`{request_id="%s"}`, reqID), 10*time.Second)
    require.NotEmpty(t, lines, "Loki must return ≥1 log line for request_id")

    // 3. Assert every returned line carries trace_id, span_id, source fields
    for _, line := range lines {
        var entry map[string]any
        require.NoError(t, json.Unmarshal([]byte(line), &entry))
        assert.Equal(t, traceID, entry["trace_id"])
        assert.NotEmpty(t, entry["span_id"])
        source := entry["source"].(map[string]any)
        assert.NotEmpty(t, source["file"])
        assert.NotEmpty(t, source["function"])
    }

    // 4. Query by trace_id — should return same lines
    byTrace := queryLoki(t, fmt.Sprintf(`{trace_id="%s"}`, traceID), 10*time.Second)
    require.NotEmpty(t, byTrace)
}
```

`queryLoki` polls `http://loki:3100/loki/api/v1/query_range` with an
exponential back-off up to the given deadline. The test runs as part of
`make test-e2e` because the full docker-compose stack (including Loki and
Promtail) is already up for the rest of the E2E suite.

---

## Verification

### Stack smoke (both tiers)

```bash
# Start the full stack
make docker-up
sleep 30   # let Promtail and the collector settle

# Tier 11: confirm OTLP paths are alive
curl -fsS http://localhost:8889/metrics | grep -c '^otelcol_'
# expected: non-zero (collector Prometheus self-stats)

# Tier 11: confirm app metrics still arrive on the app's /metrics
curl -fsS http://localhost:2112/metrics | grep -c '^http_request_duration_seconds'
# expected: non-zero

# Tier 11: confirm a trace appears in Jaeger
curl -fsS "http://localhost:16686/api/services" | jq '.data[]' | grep community-waste

# Tier 11: confirm logs arrive in Loki via BOTH paths (Promtail + OTLP bridge)
# Generate a log line
curl -fsS http://localhost:8080/health
sleep 5
curl -fsS 'http://localhost:3100/loki/api/v1/query?query=\{service_name="community-waste-collection-api"\}&limit=5' \
  | jq '.data.result[0].values[0][1]'
# expected: a JSON line with "source" key (confirms AddSource:true landed)
```

### Exemplar round-trip (T76/T77)

```bash
# Generate a few requests to populate exemplar-bearing histogram observations
for i in $(seq 1 20); do
  curl -s -X POST http://localhost:8080/api/pickups \
    -H 'Content-Type: application/json' \
    -d '{"household_id":"00000000-0000-0000-0000-000000000001","type":"organic"}' \
    > /dev/null
done

# Check exemplars are present in the Prometheus text exposition
curl -fsS http://localhost:2112/metrics \
  | grep -A3 'http_request_duration_seconds_bucket' \
  | grep '# {' | head -3
# expected: lines like  # {trace_id="…"} 0.123 1234567890
```

### Error envelope & log correlation (T83/T84/T88)

```bash
# Trigger a validation error
RESP=$(curl -si -X POST http://localhost:8080/api/pickups \
  -H 'Content-Type: application/json' \
  -d '{}')

# Check X-Request-Id is set
echo "$RESP" | grep -i x-request-id

# Check meta.trace_id is present and 32 chars
echo "$RESP" | tail -1 | jq '.meta.trace_id | length'
# expected: 32

TRACE_ID=$(echo "$RESP" | tail -1 | jq -r '.meta.trace_id')
REQUEST_ID=$(echo "$RESP" | grep -i x-request-id | awk '{print $2}' | tr -d '\r')

# Query Loki — wait up to 15s for log propagation
sleep 10
curl -fsS "http://localhost:3100/loki/api/v1/query_range" \
  --data-urlencode "query={request_id=\"$REQUEST_ID\"}" \
  --data-urlencode "start=$(date -d '1 minute ago' +%s)000000000" \
  | jq '.data.result[].values[][1]' | head -3
# expected: JSON lines with source.file, source.function, trace_id, span_id
```

### Source attribution (T83/T85)

```bash
# Confirm source fields appear in raw stdout log (AddSource:true)
docker compose logs api 2>&1 | tail -20 \
  | jq -R 'fromjson? | select(.source) | .source' | head -5
# expected: {"function":"…","file":"internal/…","line":42}

# Confirm source_file label is indexed in Loki
curl -fsS 'http://localhost:3100/loki/api/v1/labels' \
  | jq '.data[]' | grep source_file
# expected: "source_file"
```

### Automated (CI)

```bash
# Unit tests — OTel wiring
go test -race -count=1 ./internal/observability/... ./internal/handler/...

# E2E — error visibility chain (requires running stack)
go test -race -count=1 -tags e2e -run TestErrorVisibility ./test/e2e/...

# Full E2E suite — all 55+ tests must remain green
make test-e2e

# Lint — no regressions
golangci-lint run ./...

# Coverage gate must hold
make test-unit
```
