# ADR 0008 — Prometheus + Grafana with RED metrics and auto-provisioning

**Status:** Accepted

## Context

The service needs observable Rate / Error / Duration metrics for HTTP
and database layers, plus business-domain instruments for the worker
and BR-04 outcomes. Reviewers should see populated dashboards in a
single `docker compose up` boot, without manual UI clicks.

## Decision

- Use `promauto` to register metrics at package init in
  `internal/observability/metrics.go`. No registry injection is needed
  because every metric is a package-level variable.
- Follow the RED pattern for HTTP and DB layers: a counter for rate,
  a counter labelled by status for errors, and a histogram for
  duration.
- Version-control both Grafana datasources and dashboards under
  `deployments/grafana/`. Auto-provision via the Grafana provisioning
  config so panels render with real data on first boot.

## Consequences

- New metrics are one `promauto.NewXxx` declaration away — no DI
  ceremony.
- Dashboards survive container restarts; reviewers see populated
  panels immediately after `docker compose up` followed by traffic
  generation.
- Histograms use Prometheus defaults; bespoke buckets matched to real
  p99 are tracked as a tuning follow-up.
