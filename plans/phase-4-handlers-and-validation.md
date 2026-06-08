# Phase 4 — Handlers and Validation

Covers the Echo HTTP layer: route registration, request parsing, input validation,
response envelope, MIME sniffing, security headers, and per-IP rate limiting.

## Route registration

`handler.RegisterRoutes` in `internal/handler/handler.go` mounts all routes under
`/api`. Operational endpoints (`/health`, `/readyz`, `/metrics`) are mounted at root.

All 15 product endpoints:

| Method | Path | Handler |
|--------|------|---------|
| POST | /api/households | CreateHousehold |
| GET | /api/households | ListHouseholds |
| GET | /api/households/:id | GetHousehold |
| DELETE | /api/households/:id | DeleteHousehold |
| POST | /api/pickups | CreatePickup |
| GET | /api/pickups | ListPickups |
| GET | /api/pickups/:id | GetPickup |
| PUT | /api/pickups/:id/schedule | SchedulePickup |
| PUT | /api/pickups/:id/complete | CompletePickup |
| PUT | /api/pickups/:id/cancel | CancelPickup |
| POST | /api/payments | CreatePayment |
| GET | /api/payments | ListPayments |
| PUT | /api/payments/:id/confirm | ConfirmPayment |
| GET | /api/reports/household/:id | HouseholdReport |
| GET | /api/reports/summary | WasteSummary |

Operational endpoints: `GET /health`, `GET /readyz`, `GET /metrics`.

## Request parsing and validation

Each handler calls `e.Bind(&req)` for JSON bodies. Domain-specific validation
(e.g., negative amounts, unknown status values, invalid dates) is done inline
before calling the service. Invalid input returns `ErrValidation` → HTTP 400.

## Response envelope

All responses use a consistent JSON envelope:

```json
{"success": true, "data": {...}}           // 200/201
{"success": true, "data": [...], "meta": {"page":1,"per_page":20,"total":5,"total_pages":1}} // list
{"success": false, "error": "message"}    // 4xx/5xx
```

`internal/handler/handler.go:respond` and `respondError` build the envelope.

## Security headers and CORS

`internal/handler/handler.go` applies to every response:

- `CORSWithConfig` with empty `AllowOrigins` (blocks cross-origin by default).
- `SecureWithConfig`: `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`,
  `Strict-Transport-Security: max-age=31536000`, `Content-Security-Policy: default-src 'none'`,
  `Referrer-Policy: no-referrer`.

## MIME allowlist and magic-byte sniff (BR-06)

`internal/handler/payment.go:ConfirmPayment`:

1. Reads `Content-Type` from the multipart part header.
2. Checks it against `allowedProofMIME` (`image/jpeg`, `image/png`, `application/pdf`).
3. Reads up to 512 bytes from the file, calls `http.DetectContentType`, re-checks against
   the allowlist.
4. Rewinds via `io.MultiReader(bytes.NewReader(sniffBuf[:n]), file)` so the full body
   reaches the service layer.

This prevents MIME-type lying (e.g., HTML served as `image/jpeg`).

## Per-IP rate limiting

`internal/middleware/ratelimit.go` implements a token-bucket rate limiter keyed on
`X-Real-IP` / `RemoteAddr` using a `sync.Map`.

- `lastSeenNano` is stored as `int64` with atomic load/store to avoid data races.
- An eviction goroutine prunes entries inactive for `RATE_LIMIT_TTL` (default: 5m).
- Rate: `RATE_LIMIT_RPS` refill/s with `RATE_LIMIT_BURST` burst capacity.
- Exceeded → `ErrRateLimit` → HTTP 429.

Applied selectively to high-frequency write endpoints (`POST /api/pickups`).

## Verification

- `go test ./internal/handler/...` — full handler test suite including 400/404/409
  cases for all 15 endpoints.
- `internal/handler/payment_test.go` — MIME allowlist + magic-byte tests.
- `go test -race ./internal/middleware/...` — confirms no race on `lastSeenNano`.
