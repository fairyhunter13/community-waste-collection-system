# Constraint

* [The codecov ignore list has to mirror the COVERPKG filter](codecov-ignore-mirrors-the-coverpkg-filter.md) - The Makefile drops internal/observability from -coverpkg, so no coverage data is produced for it at all. Removing it from codecov.yml does not measure it; it reports it as uncovered.
* [The coverage gate reads the unit profile alone, because two profiles cannot be concatenated](coverage-profiles-cannot-be-concatenated.md) - A profile starts with its own mode header, so cat of two files gives go tool cover two headers and it fails silently. The unit and integration jobs also use different -coverpkg lists.
* [An E2E test that sends a burst needs its own client IP](an-e2e-burst-test-needs-its-own-client-ip.md) - The limiter buckets on c.RealIP(), and every E2E test shares 127.0.0.1. A test that spends the burst makes an unrelated later test fail with 429.
