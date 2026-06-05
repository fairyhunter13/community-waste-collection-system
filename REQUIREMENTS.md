# Community Waste Collection API

A REST API service that manages households, waste pickup requests, and payments for community waste collection services.

---

## Tech Stack

- **Language:** Go
- **Database:** PostgreSQL
- **HTTP Framework:** Gin, Echo, Chi, net/http, etc. (open choice)
- **File Storage:** S3-compatible (implementation approach is open choice)
- **Containerization:** Docker + docker-compose (required)
- **API Tools:** Postman or Insomnia

---

## Project Overview

The system manages households, their waste pickup requests, and payments for collection services. Architecture and project structure are intentionally left open.

---

## Entities

### 1. Household

| Field | Type | Notes |
|---|---|---|
| `id` | UUID | Primary key |
| `owner_name` | string | Required |
| `address` | string | Required |
| `created_at` | timestamp | |
| `updated_at` | timestamp | |

### 2. Waste Pickup

| Field | Type | Notes |
|---|---|---|
| `id` | UUID | Primary key |
| `household_id` | UUID | FK → Household |
| `type` | enum | `organic`, `plastic`, `paper`, `electronic` |
| `status` | enum | `pending`, `scheduled`, `completed`, `canceled` |
| `pickup_date` | timestamp | Nullable |
| `safety_check` | boolean | Required for `electronic` type only |
| `created_at` | timestamp | |
| `updated_at` | timestamp | |

### 3. Payment

| Field | Type | Notes |
|---|---|---|
| `id` | UUID | Primary key |
| `household_id` | UUID | FK → Household |
| `waste_id` | UUID | FK → Waste Pickup |
| `amount` | decimal | Required |
| `payment_date` | timestamp | Nullable |
| `status` | enum | `pending`, `paid`, `failed` |
| `proof_file_url` | string | Nullable, populated on confirmation |
| `created_at` | timestamp | |
| `updated_at` | timestamp | |

---

## API Endpoints

### 1. Household API

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/households` | Create new household |
| `GET` | `/api/households` | List households with pagination |
| `GET` | `/api/households/:id` | Get household detail |
| `DELETE` | `/api/households/:id` | Delete household |

### 2. Waste Pickup API

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/pickups` | Create new pickup request (`type` required) |
| `GET` | `/api/pickups` | List pickups, filter by `status` and `household_id` |
| `PUT` | `/api/pickups/:id/schedule` | Schedule pickup — set `pickup_date`, update status |
| `PUT` | `/api/pickups/:id/complete` | Mark pickup as completed |
| `PUT` | `/api/pickups/:id/cancel` | Cancel pickup |

### 3. Payment API

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/payments` | Create payment linked to household |
| `GET` | `/api/payments` | List payments, filter by status, household, and date range |
| `PUT` | `/api/payments/:id/confirm` | Confirm payment with proof of payment file upload (S3-compatible) |

### 4. Reporting API

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/reports/waste-summary` | Pickups aggregated by type and status |
| `GET` | `/api/reports/payment-summary` | Total payments by status and total revenue |
| `GET` | `/api/reports/households/:id/history` | Full pickup and payment history for a household |

---

## Business Rules

1. A household **cannot create a new pickup request** if they have any payment with `pending` status.
2. A pickup can only be **scheduled** if its current status is `pending`.
3. **Electronic** type pickups cannot be scheduled unless `safety_check` is `true`.
4. **Organic** type pickups must be **auto-canceled** if not picked up within **3 days** of creation.
   - Implement as a background goroutine that shuts down cleanly on application exit (graceful shutdown).
5. Once a pickup is **completed**, automatically generate a payment record:
   - `organic`, `plastic`, `paper` → `amount = 50,000`
   - `electronic` → `amount = 100,000`
6. **Payment confirmation** requires uploading a proof of payment file to an S3-compatible storage. The resulting file URL must be saved to `proof_file_url` on the payment record.

---

## Technical Requirements

1. Dependency injection
2. Graceful shutdown
3. Rate limiting on pickup creation
4. Docker — app + PostgreSQL, launchable with a single command
5. Consistent API responses with appropriate HTTP status codes
6. Input validation

---

## Deliverables

1. Go project with PostgreSQL configured and running via Docker
2. Source code with a clear, reasoned project structure
3. Postman or Insomnia collection covering all endpoints
4. `README.md` containing:
   - Setup and run instructions
   - How to run migrations and seeding
   - Environment variable reference
   - Brief explanation of architecture decisions
