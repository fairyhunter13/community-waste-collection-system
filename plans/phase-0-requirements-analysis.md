# Phase 0 — Requirements Gathering & Analysis

## Purpose

Capture the complete, unambiguous specification of the system before any design or code begins. This document is the single source of truth for **what** the system does — not **how**. Every implementation decision in later phases must trace back to a requirement here.

---

## 1. Domain Entities

### 1.1 Household

| Field | Type | Constraints | Notes |
|---|---|---|---|
| `id` | UUID | PK, not null, auto-generated | `gen_random_uuid()` |
| `owner_name` | text | not null, min 1 char | |
| `address` | text | not null, min 1 char | |
| `created_at` | timestamptz | not null, default now() | Set on INSERT, never updated |
| `updated_at` | timestamptz | not null, default now() | Updated on every modification |

**Invariants:**
- A household is identified by its UUID; `owner_name` is not unique across households
- Deleting a household cascades deletion to its pickups and payments (ON DELETE CASCADE)

---

### 1.2 Waste Pickup

| Field | Type | Constraints | Notes |
|---|---|---|---|
| `id` | UUID | PK, not null, auto-generated | |
| `household_id` | UUID | FK → households.id, not null | |
| `type` | enum | not null | `organic`, `plastic`, `paper`, `electronic` |
| `status` | enum | not null, default `pending` | `pending`, `scheduled`, `completed`, `canceled` |
| `pickup_date` | timestamptz | nullable | Set when scheduled; null until then |
| `safety_check` | boolean | not null, default false | Required to be true before scheduling `electronic` |
| `created_at` | timestamptz | not null, default now() | |
| `updated_at` | timestamptz | not null, default now() | |

**Valid status transitions:**
```
pending   → scheduled   (schedule action; BR-02, BR-03 must pass)
pending   → canceled    (cancel action or auto-canceled by worker)
scheduled → completed   (complete action; triggers BR-05)
scheduled → canceled    (cancel action)
completed → (terminal — no further transitions allowed)
canceled  → (terminal — no further transitions allowed)
```

**Invariants:**
- `type` is immutable after creation
- `safety_check` may be set at creation; a false value on an electronic pickup does not block creation, only scheduling
- `pickup_date` is only meaningful when status is `scheduled` or `completed`

---

### 1.3 Payment

| Field | Type | Constraints | Notes |
|---|---|---|---|
| `id` | UUID | PK, not null, auto-generated | |
| `household_id` | UUID | FK → households.id, not null | Denormalized for efficient household-level queries |
| `waste_id` | UUID | FK → waste_pickups.id, not null, UNIQUE | One payment per pickup; enforced at DB level |
| `amount` | numeric(12,2) | not null, > 0 | Never use floating point for monetary values |
| `payment_date` | timestamptz | nullable | Set when status transitions to `paid` |
| `status` | enum | not null, default `pending` | `pending`, `paid`, `failed` |
| `proof_file_url` | text | nullable | Populated on payment confirmation via S3 upload |
| `created_at` | timestamptz | not null, default now() | |
| `updated_at` | timestamptz | not null, default now() | |

**Valid status transitions:**
```
pending → paid    (confirm action with proof file upload)
pending → failed  (system or manual failure)
paid    → (terminal)
failed  → (terminal; retry not in scope)
```

**Invariants:**
- Amount is set at payment creation and is immutable
- `proof_file_url` is only set when transitioning to `paid`
- `payment_date` is set to the current timestamp when transitioning to `paid`

---

## 2. Business Rules

### BR-01 — Pending Payment Blocks New Pickup

**Statement:** A household cannot create a new pickup request if any of its payments has `pending` status.

| Attribute | Value |
|---|---|
| Trigger | `POST /api/pickups` |
| Check | `SELECT EXISTS(SELECT 1 FROM payments WHERE household_id = $1 AND status = 'pending')` |
| Failure condition | Result is true |
| HTTP response | 409 Conflict |
| Error code | `CONFLICT` |
| Message | `"household has a pending payment"` |

**Edge cases:**
- A household with no payments at all is **not** blocked
- A household with only `paid` or `failed` payments is **not** blocked
- Multiple pending payments still count as one block (any pending = blocked)

---

### BR-02 — Schedule Requires Pending Status

**Statement:** A pickup can only be scheduled if its current status is `pending`.

| Attribute | Value |
|---|---|
| Trigger | `PUT /api/pickups/:id/schedule` |
| Check | pickup.status == pending |
| Failure condition | status is `scheduled`, `completed`, or `canceled` |
| HTTP response | 409 Conflict |
| Error code | `CONFLICT` |
| Message | `"pickup cannot be scheduled: current status is {status}"` |

---

### BR-03 — Electronic Scheduling Requires Safety Check

**Statement:** A pickup of type `electronic` cannot be scheduled unless `safety_check` is `true`.

| Attribute | Value |
|---|---|
| Trigger | `PUT /api/pickups/:id/schedule` |
| Check | pickup.type == electronic → pickup.safety_check == true |
| Failure condition | type is electronic AND safety_check is false |
| HTTP response | 422 Unprocessable Entity |
| Error code | `BUSINESS_RULE_VIOLATION` |
| Message | `"electronic pickup requires safety_check to be true before scheduling"` |

**Note:** BR-02 and BR-03 are both checked on schedule. BR-02 takes precedence (check status first).

---

### BR-04 — Organic Auto-Cancel After 3 Days

**Statement:** Organic pickups that remain in `pending` status for more than 3 days from their `created_at` timestamp must be automatically canceled by a background process.

| Attribute | Value |
|---|---|
| Trigger | Background goroutine running on configurable interval (default: 1 hour) |
| Check | `type = 'organic' AND status = 'pending' AND created_at < NOW() - INTERVAL '3 days'` |
| Action | Bulk UPDATE status = 'canceled', updated_at = NOW() |
| Shutdown behavior | Worker goroutine must exit cleanly when a cancellation context is received |

**Edge cases:**
- A pickup created exactly at the 3-day boundary may or may not be canceled depending on tick timing; this is acceptable
- Cancellation is irreversible; no retry for auto-canceled pickups
- Worker failure (DB error) must be logged but must not crash the application

---

### BR-05 — Completed Pickup Auto-Generates Payment

**Statement:** When a pickup is marked as completed, a payment record must be automatically created within the same database transaction.

| Pickup Type | Payment Amount |
|---|---|
| `organic` | 50,000.00 |
| `plastic` | 50,000.00 |
| `paper` | 50,000.00 |
| `electronic` | 100,000.00 |

| Attribute | Value |
|---|---|
| Trigger | `PUT /api/pickups/:id/complete` |
| Action | In a single DB transaction: UPDATE pickup status → 'completed'; INSERT payment record |
| Failure | If payment insertion fails, entire transaction rolls back; pickup status remains unchanged |

**Note:** Only `scheduled` pickups can be completed. Status must be `scheduled` before calling complete (enforce this check before beginning the transaction).

---

### BR-06 — Payment Confirmation Requires File Upload

**Statement:** Confirming a payment requires uploading a proof-of-payment file to S3-compatible storage. The resulting file URL must be stored in `proof_file_url` on the payment record.

| Attribute | Value |
|---|---|
| Trigger | `PUT /api/payments/:id/confirm` |
| Request | `multipart/form-data` with field name `proof` |
| Accepted types | `image/jpeg`, `image/png`, `application/pdf` |
| Max file size | 10 MB |
| Action | Upload file to S3; update payment: status = 'paid', proof_file_url = URL, payment_date = NOW() |
| Failure | Missing file → 400; S3 upload failure → 500; payment already paid → 409 |

---

## 3. API Contract

### 3.1 Universal Response Envelope

All endpoints return one of three shapes:

**Success — single object:**
```json
{
  "success": true,
  "data": { ... }
}
```

**Success — list with pagination:**
```json
{
  "success": true,
  "data": [ ... ],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 100,
    "total_pages": 5
  }
}
```

**Error:**
```json
{
  "success": false,
  "error": {
    "code": "CONFLICT",
    "message": "household has a pending payment"
  }
}
```

**Error codes:**

| Code | HTTP Status | When |
|---|---|---|
| `VALIDATION_ERROR` | 400 | Malformed request body or missing required fields |
| `NOT_FOUND` | 404 | Resource does not exist |
| `CONFLICT` | 409 | State transition not allowed (BR-01, BR-02) |
| `BUSINESS_RULE_VIOLATION` | 422 | Business rule blocks the action (BR-03) |
| `RATE_LIMITED` | 429 | Too many requests |
| `INTERNAL_ERROR` | 500 | Unexpected server error |

---

### 3.2 Household Endpoints

#### `POST /api/households`
Create a new household.

Request body:
```json
{
  "owner_name": "John Doe",
  "address": "Jl. Merdeka No. 1, Jakarta"
}
```

Response `201 Created`:
```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "owner_name": "John Doe",
    "address": "Jl. Merdeka No. 1, Jakarta",
    "created_at": "2026-06-05T09:00:00Z",
    "updated_at": "2026-06-05T09:00:00Z"
  }
}
```

Errors: `400` (validation), `500`

---

#### `GET /api/households`
List households with pagination.

Query parameters: `page` (int, default 1), `per_page` (int, default 20, max 100)

Response `200 OK`: list envelope with household array.

---

#### `GET /api/households/:id`
Get a single household by ID.

Response `200 OK`: single household object.
Errors: `404`

---

#### `DELETE /api/households/:id`
Delete a household and all its associated data (CASCADE).

Response `204 No Content`: no body.
Errors: `404`

---

### 3.3 Waste Pickup Endpoints

#### `POST /api/pickups`
Create a new pickup request.

Request body:
```json
{
  "household_id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "organic",
  "safety_check": false
}
```

- `type`: required; one of `organic`, `plastic`, `paper`, `electronic`
- `safety_check`: optional boolean, defaults to false

Response `201 Created`: pickup object with status `pending`.
Errors: `400` (validation or non-existent household_id), `409` (BR-01), `429` (rate limited)

---

#### `GET /api/pickups`
List pickups with optional filters.

Query parameters: `household_id` (UUID, optional), `status` (optional), `page`, `per_page`

Response `200 OK`: list envelope.

---

#### `PUT /api/pickups/:id/schedule`
Schedule a pickup by setting its date and transitioning to `scheduled`.

Request body:
```json
{
  "pickup_date": "2026-06-10T09:00:00Z"
}
```

- `pickup_date`: required ISO 8601 timestamp

Response `200 OK`: updated pickup object.
Errors: `400` (missing pickup_date), `404`, `409` (BR-02), `422` (BR-03)

---

#### `PUT /api/pickups/:id/complete`
Mark a pickup as completed. Atomically creates a payment record (BR-05).

Request body: none.

Response `200 OK`: updated pickup object.
Errors: `404`, `409` (status not `scheduled`)

---

#### `PUT /api/pickups/:id/cancel`
Cancel a pickup.

Request body: none.

Response `200 OK`: updated pickup object.
Errors: `404`, `409` (status is `completed` or already `canceled`)

---

### 3.4 Payment Endpoints

#### `POST /api/payments`
Manually create a payment linked to a household and pickup.

Request body:
```json
{
  "household_id": "uuid",
  "waste_id": "uuid",
  "amount": "50000.00"
}
```

Response `201 Created`: payment object.
Errors: `400` (validation or non-existent household_id/waste_id), `409` (payment already exists for this pickup)

---

#### `GET /api/payments`
List payments with optional filters.

Query parameters: `household_id` (UUID), `status`, `date_from` (ISO 8601), `date_to` (ISO 8601), `page`, `per_page`

Response `200 OK`: list envelope.

---

#### `PUT /api/payments/:id/confirm`
Confirm a payment with proof-of-payment file upload (BR-06).

Request: `multipart/form-data` with field `proof` (file).

Response `200 OK`:
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "status": "paid",
    "proof_file_url": "http://minio:9000/waste-proofs/uuid/proof.jpg",
    "payment_date": "2026-06-05T10:00:00Z",
    ...
  }
}
```

Errors: `400` (no file or wrong type), `404`, `409` (already paid), `500` (S3 failure)

---

### 3.5 Reporting Endpoints

#### `GET /api/reports/waste-summary`
Aggregated pickup counts by type and status.

Response `200 OK`:
```json
{
  "success": true,
  "data": {
    "by_type": [
      {
        "type": "organic",
        "total": 42,
        "by_status": {
          "pending": 10,
          "scheduled": 5,
          "completed": 25,
          "canceled": 2
        }
      },
      { "type": "plastic", ... },
      { "type": "paper", ... },
      { "type": "electronic", ... }
    ]
  }
}
```

---

#### `GET /api/reports/payment-summary`
Total payment counts by status and total revenue from paid payments.

Response `200 OK`:
```json
{
  "success": true,
  "data": {
    "by_status": {
      "pending": 10,
      "paid": 45,
      "failed": 2
    },
    "total_revenue": "2250000.00"
  }
}
```

---

#### `GET /api/reports/households/:id/history`
Full pickup and payment history for a specific household.

Response `200 OK`:
```json
{
  "success": true,
  "data": {
    "household": { ... },
    "pickups": [ ... ],
    "payments": [ ... ]
  }
}
```

Errors: `404` (household not found)

---

## 4. Non-Functional Requirements

| Requirement | Target | Notes |
|---|---|---|
| Rate limit (pickup creation) | 5 req/sec per IP, burst 10 | Token bucket via `golang.org/x/time/rate` |
| Max file upload size | 10 MB | Enforced before S3 upload |
| Accepted proof file types | `image/jpeg`, `image/png`, `application/pdf` | Validate Content-Type header |
| Graceful shutdown timeout | 10 seconds | After SIGINT/SIGTERM |
| DB connection pool (max open) | 25 | Configurable via env |
| DB connection pool (max idle) | 10 | Configurable via env |
| DB connection idle timeout | 5 minutes | Configurable via env |
| Organic auto-cancel interval | 1 hour | Configurable via env |
| pprof debug endpoint | `:6060/debug/pprof/` | Internal only; never expose publicly |
| Prometheus metrics endpoint | `:2112/metrics` | Internal; scrape by Prometheus |
| OTel trace export | OTLP HTTP/gRPC | Endpoint configurable via env |

---

## 5. Constraints & Assumptions

- **No authentication/authorization** — out of scope; all endpoints are open
- **Single-instance deployment** — rate limiting is in-memory (no Redis needed)
- **PostgreSQL 17** — specific dialect features used (gen_random_uuid, ENUM types, NUMERIC, TIMESTAMPTZ)
- **All timestamps** are stored and returned as UTC (TIMESTAMPTZ)
- **UUIDs generated by PostgreSQL** (`gen_random_uuid()`), not by the application layer
- **Amount stored as NUMERIC(12,2)** — never FLOAT/DOUBLE for monetary values
- **S3-compatible storage** — MinIO locally; configurable via env for other providers
- **proof_file_url** stores the fully qualified URL to the uploaded file
- **Daily commits** are expected throughout the development period

---

## 6. Domain Glossary

| Term | Definition |
|---|---|
| Household | A registered address and owner that can request waste collection services |
| Waste Pickup | A scheduled or requested event to collect waste from a household |
| Payment | A financial record linked to a completed pickup, requiring confirmation |
| Safety Check | A boolean flag indicating a pre-pickup safety inspection was completed (electronic waste only) |
| Proof of Payment | A file (image/PDF) uploaded as evidence that a payment was made |
| Auto-Cancel | Automatic status transition to `canceled` for overdue organic pickups by the background worker |
| Organic Cutoff | The maximum age (3 days) a pending organic pickup can exist before being auto-canceled |
