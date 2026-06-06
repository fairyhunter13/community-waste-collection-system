# ADR 0004 — Per-IP token bucket rate limiting

**Status:** Accepted

## Context

The spec requires rate limiting on `POST /api/pickups` to protect the
DB-write path. Bringing in Redis or another external store solely for
rate limiting would inflate the deployment footprint of a single-node
service.

## Decision

Use `golang.org/x/time/rate` to maintain a per-IP `rate.Limiter` in a
`sync.Map`. Configuration (rps + burst) lives in environment variables
so the limit can be tuned per environment without redeploying.

## Consequences

- Zero new infrastructure dependencies for the spec's rate-limit
  requirement.
- Limits are per-process: behind a load balancer, capacity scales with
  replica count. That is the expected single-tenant deployment shape
  for this service.
- The `sync.Map` grows unbounded by IP key; eviction of idle entries
  is needed to bound memory. That follow-up is tracked separately
  rather than blocking initial delivery.
