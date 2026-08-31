---
type: Constraint
resource: internal/middleware/ratelimit.go
title: An E2E test that sends a burst needs its own client IP
description: The limiter buckets on c.RealIP(), and every E2E test shares 127.0.0.1. A test that spends the burst leaves the shared bucket empty, so an unrelated test that runs later fails with 429.
tags: [ratelimit, e2e, middleware, test-isolation]
generated: { by: claude/opus-5, at: 2026-08-31T12:00:00Z }
---

The rate limit middleware keys its buckets on `c.RealIP()`. The E2E suite talks to the app over
loopback, so every test in it shares one bucket unless it says otherwise.

`TestPickup_RateLimit` sets `X-Real-IP: 192.0.2.1` for its own requests. That is not cosmetic. Echo
reads the header for `RealIP()`, so the test gets a bucket nobody else touches, and its 60-request
burst cannot drain the one the rest of the suite is drawing from.

The failure this prevents is not local. It surfaces as a 429 in some later, unrelated test, and it
depends on suite order, so it does not reproduce when that test is run alone. Any new test that
sends requests in bulk has to pick its own address from the `192.0.2.0/24` documentation range.
