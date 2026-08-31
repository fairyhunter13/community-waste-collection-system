---
type: Log
title: community-waste-collection-system knowledge history
---

# Bundle history

## 2026-08-31

- **Creation**: three concepts harvested from the 126 commit bodies in the repo. [A smoke test that asserted what never fails](defects/the-smoke-test-asserted-what-never-fails.md), because the ordering bug it hid is fixed and the assertion pattern is not. [A derived document is deleted, not maintained](policies/a-derived-document-is-deleted-not-maintained.md), which one commit applied and a later one applied again against its own earlier exception. [The codecov ignore mirrors the COVERPKG filter](constraints/codecov-ignore-mirrors-the-coverpkg-filter.md), which was changed once in the wrong direction and changed back.

- **Creation**: candidates rejected. `SpanFail` carries its own reason in a three-line doc comment on `internal/observability/tracing.go`, so a concept would restate code a reader can open. The Go toolchain bump to 1.26.4 and the GitHub Actions major bumps are dated facts that `go.mod` and `ci.yml` hold exactly. The coverage threshold at 81 % is a number in the Makefile with no argument a reader would re-derive.

- **Creation**: gate wired as `make lint-knowledge`, which `test` depends on, following the repo's own Makefile convention. `lint-knowledge-strict` runs the `-Werror` form and nothing depends on it, because the bundle is small enough that a forward link is likely while it grows.
