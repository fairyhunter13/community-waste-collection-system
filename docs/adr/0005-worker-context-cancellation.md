# ADR 0005 — Background worker with context cancellation

**Status:** Accepted

## Context

BR-04 requires the system to auto-cancel pending organic pickups that
exceed a cutoff age. This is naturally a periodic background task, not
something to run on the request path. We also need the process to shut
down cleanly under SIGINT / SIGTERM without losing in-flight cycles or
leaking goroutines.

## Decision

Implement `internal/worker/OrganicCanceler` with a `time.Ticker` and a
single `context.Context` parameter. The main function creates a
cancellable context, launches the worker in a goroutine, and waits on
`sync.WaitGroup` after sending the cancel signal.

## Consequences

- Shutdown is deterministic: receive signal → cancel ctx → ticker
  loop sees `<-ctx.Done()` → `wg.Wait()` blocks the process until the
  worker returns.
- Adding new background workers follows the same pattern (ctx in,
  goroutine launch, `wg.Add(1) / wg.Done()` discipline).
- Failure isolation requires care: a panic in the loop body would kill
  the goroutine silently. Panic recovery is tracked separately.
