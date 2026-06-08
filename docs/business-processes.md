# Business Processes

End-to-end flows and state machines for the core business rules. Each
diagram is paired with the code path that enforces it.

---

## Pickup Lifecycle

Every pickup moves through a defined set of states. Business rules gate
each transition.

```mermaid
stateDiagram-v2
    [*] --> pending : create pickup
    pending --> scheduled : schedule
    pending --> canceled : cancel
    pending --> canceled : BR-04 auto-cancel
    scheduled --> completed : complete
    scheduled --> canceled : cancel
    completed --> [*]
    canceled --> [*]
```

**Enforcement:** `internal/service/pickup.go` — each transition uses a
conditional `UPDATE … WHERE status = <expected>` that returns `ErrConflict`
when the row is already in a different state (BR-02 safety net).

---

## Payment Lifecycle

Payments are created automatically when a pickup completes (BR-05) and
confirmed by uploading a proof file (BR-06).

```mermaid
stateDiagram-v2
    [*] --> pending : auto-created on complete
    pending --> paid : confirm with proof
    pending --> failed : admin action
    paid --> [*]
    failed --> [*]
```

**Enforcement:** `internal/service/payment.go:Confirm` — uploads the
multipart proof file to MinIO, then performs a conditional DB update. On
storage success + DB failure the uploaded object is deleted as best-effort
cleanup.

---

## Pickup Creation — BR-01 Gate

A household cannot have a new pickup created while a pending payment
exists for it. The gate is enforced at both the service layer and the DB.

```mermaid
flowchart TD
    A[POST /api/pickups] --> B{HasPendingPaymentForHousehold?}
    B -- Yes --> C[409 Conflict<br/>BR-01 violation]
    B -- No --> D{acquire pg_advisory_xact_lock<br/>household_id hash}
    D --> E{re-check pending payment<br/>inside transaction}
    E -- Yes --> F[rollback + 409]
    E -- No --> G[INSERT waste_pickup<br/>with partial-UNIQUE guard]
    G --> H[201 Created]
```

**Enforcement layers:**
1. `service/pickup.go:Create` — `HasPendingPaymentForHousehold` query before the advisory lock.
2. `pg_advisory_xact_lock` — serialises concurrent creates for the same household.
3. Partial UNIQUE index `uq_pickups_pending_per_household` — DB-level safety net for any concurrent bypass.

---

## Complete Pickup — BR-05 Atomic Transaction

Completing a pickup and creating its payment record happens inside a
single database transaction. Either both succeed or neither does.

```mermaid
sequenceDiagram
    autonumber
    participant S as Service
    participant R as Repository
    participant DB as PostgreSQL

    S->>DB: BEGIN tx
    S->>R: UpdateStatus(tx, id, pending_payment=false)<br/>WHERE status='scheduled'
    R->>DB: UPDATE waste_pickups SET status='completed'<br/>WHERE id=? AND status='scheduled'
    DB-->>R: rows affected (1 = OK, 0 = conflict)
    alt rows affected == 0
        R-->>S: ErrConflict
        S->>DB: ROLLBACK
    else rows affected == 1
        S->>R: CreateWithTx(tx, payment)
        R->>DB: INSERT INTO payments<br/>(partial-UNIQUE index guard)
        DB-->>R: payment row
        R-->>S: payment
        S->>DB: COMMIT
        S-->>S: return completed pickup + new payment
    end
```

**Code:** `internal/service/pickup.go:Complete` (lines 182–264).

---

## Payment Confirm — BR-06 Proof Upload Flow

Confirming a payment requires a valid proof file. The handler enforces
the MIME allowlist and magic-byte check before the service uploads to S3.

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant H as Handler
    participant S as Service
    participant M as MinIO/S3
    participant R as Repository
    participant DB as PostgreSQL

    C->>H: PUT /api/payments/:id/confirm<br/>multipart/form-data proof file
    H->>H: check Content-Type in allowlist<br/>(image/jpeg, image/png, application/pdf)
    H->>H: sniff magic bytes (FF D8 FF, 89 PNG, 25 50 44 46)
    alt invalid MIME or magic bytes
        H-->>C: 400 Bad Request
    else valid
        H->>S: Confirm(id, reader, size, contentType)
        S->>M: PutObject(bucket, key, reader)
        alt S3 upload fails
            M-->>S: error
            S-->>H: ErrValidation
            H-->>C: 400
        else S3 upload succeeds
            M-->>S: object URL
            S->>R: Confirm(id, proofURL, paidAt)
            R->>DB: UPDATE payments SET status='paid'<br/>proof_file_url=? WHERE id=? AND status='pending'
            alt DB update fails
                DB-->>R: error
                R-->>S: error
                S->>M: DeleteObject (best-effort cleanup)
                S-->>H: error
                H-->>C: 500
            else DB update succeeds
                DB-->>R: paid payment row
                R-->>S: payment
                S-->>H: payment
                H-->>C: 200 OK
            end
        end
    end
```

**Code:** `internal/handler/payment.go:104-163` (MIME + magic-byte check),
`internal/service/payment.go:Confirm` (S3 upload + DB update + cleanup).

---

## BR-04 Worker — Organic Auto-Cancel

A background goroutine periodically cancels organic pickups that were
never scheduled within the configured cutoff window.

```mermaid
sequenceDiagram
    autonumber
    participant M as main.go
    participant W as Worker goroutine
    participant R as Repository
    participant DB as PostgreSQL

    M->>W: go worker.Run(ctx)
    loop every WORKER_CANCEL_INTERVAL
        W->>R: CancelExpiredOrganicPickups(ctx, cutoffTime)
        R->>DB: UPDATE waste_pickups<br/>SET status='canceled'<br/>WHERE type='organic' AND status='pending'<br/>AND created_at < now() - cutoff
        DB-->>R: rows affected
        R-->>W: count canceled
        W->>W: log count + emit metric
    end
    Note over M,W: SIGTERM received
    M->>W: context.Cancel()
    W->>W: ticker.Stop(), drain in-flight tick
    W-->>M: goroutine exits
    Note over M: wg.Wait() unblocks, graceful shutdown proceeds
```

**Code:** `internal/worker/organic_canceler.go`. Context cancellation is
handled inside the `for range ticker.C` loop; in-flight DB queries carry
the same context and return promptly when cancelled.
