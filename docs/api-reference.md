# API Reference

All 15 product endpoints, the response envelope contract, and links to
the machine-readable API artefacts.

---

## Endpoint Map

```mermaid
graph TD
    subgraph Households
        H1["POST /api/households\nCreate"]
        H2["GET /api/households\nList"]
        H3["GET /api/households/:id\nGet"]
        H4["DELETE /api/households/:id\nDelete + cascade"]
    end

    subgraph Pickups
        P1["POST /api/pickups\nCreate (rate-limited)"]
        P2["GET /api/pickups\nList + filter"]
        P3["PUT /api/pickups/:id/schedule\nSchedule"]
        P4["PUT /api/pickups/:id/complete\nComplete (auto-payment)"]
        P5["PUT /api/pickups/:id/cancel\nCancel"]
    end

    subgraph Payments
        PA1["POST /api/payments\nCreate"]
        PA2["GET /api/payments\nList + filter"]
        PA3["PUT /api/payments/:id/confirm\nConfirm with proof upload"]
    end

    subgraph Reports
        R1["GET /api/reports/waste-summary\nAggregate by type + status"]
        R2["GET /api/reports/payment-summary\nAggregate by status + revenue"]
        R3["GET /api/reports/households/:id/history\nPickup + payment history"]
    end
```

Extra (not in the 15): `GET /health` (liveness), `GET /readyz` (readiness
with DB ping).

---

## Response Envelope

Every response uses the same JSON envelope. The handler helpers
`respond`, `respondError`, and `respondList` in
`internal/handler/handler.go` enforce this contract.

### Success — single object

```json
{
  "success": true,
  "data": { ... }
}
```

### Success — collection

```json
{
  "success": true,
  "data": [ ... ],
  "meta": {
    "total": 42,
    "page": 1,
    "per_page": 20
  }
}
```

### Error

```json
{
  "success": false,
  "code": "validation_error",
  "message": "owner_name is required"
}
```

| HTTP status | `code` value | Trigger |
|---|---|---|
| 400 | `validation_error` | Input fails validator.v10 or body-limit exceeded |
| 404 | `not_found` | Resource does not exist |
| 409 | `conflict` | BR-01 pending payment, BR-02 wrong status |
| 413 | `request_too_large` | Body exceeds `MAX_UPLOAD_SIZE_MB` |
| 422 | `business_rule_violation` | BR-03 electronic safety check |
| 429 | `rate_limit_exceeded` | Pickup creation rate limit hit |
| 500 | `internal_error` | Unexpected server error |

---

## Rate Limiting

`POST /api/pickups` is the only rate-limited endpoint. The limit is
per-IP, enforced by a token-bucket rate limiter in
`internal/middleware/ratelimit.go`.

Environment variables: `RATE_LIMIT_RPS` (tokens per second),
`RATE_LIMIT_BURST` (maximum burst).

---

## API Artefacts

| Artefact | Path | Used by |
|---|---|---|
| OpenAPI 3.0 spec | `api/openapi.yaml` | Redocly lint in `contract` CI job; client generation |
| Postman collection | `api/community-waste.postman_collection.json` | Newman smoke test in `e2e` CI job; manual testing |
| Insomnia collection | `api/community-waste.insomnia_collection.json` | Manual testing |

Both Postman and Insomnia collections have 27 requests each. The `contract`
CI job (`ci.yml`) verifies they stay in sync via a Python count check.
