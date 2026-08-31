---
type: Policy
resource: docs/architecture.md
title: A document that restates something CI checks is deleted, not maintained
description: Eight docs described one three-entity service. Four were derived from openapi.yaml, `make coverage` or the code itself, so they were removed rather than re-synced, and README fell from 812 lines to 334.
tags: [documentation, ci, duplication]
generated: { by: claude/opus-5, at: 2026-08-31T12:00:00Z }
---

# The test a document has to pass

Ask what checks the claim. Where the answer is a file CI already reads, the document is a second
copy that drifts alone, and the drift is invisible until somebody trusts the copy.

Four documents failed that test and went:

* `docs/plans/`, 579 lines of a seven-phase build plan whose every phase is in the code, and which
  nothing referenced.
* `coverage-matrix.md`, 355 hand-maintained lines of what `make coverage` regenerates.
* `api-reference.md`, a restatement of `api/openapi.yaml` — the same endpoint map, the same
  `ErrorResponse` schema, the same `RATE_LIMITED`, in 1,211 machine-checked lines.
* `docs/README.md`, an index of the three above.

`business-processes.md` and `data-model.md` passed. They hold the state machines and the ER
diagram, which no file in the repo derives, so they folded into `docs/architecture.md` rather than
going.

# The same test settled a script twice, in opposite directions

`scripts/verify-collections.sh` chained the three collection checks. It was kept once, on the
argument that it is a documented hand-run gate of five layers where CI runs three separately. It
was deleted later, on the argument that won: nothing called it, and it was a fourth written copy of
a running order that no gate read.

The lesson is the order of the two arguments. *Useful to run by hand* does not beat *no caller*.
`lint_collections.py`, `verify_postman_load.js` and `verify_insomnia_load.js` stayed, because
`.github/workflows/ci.yml` runs each of them directly.

# Why this is a policy and not a preference

The three earlier documentation commits in this repo were all corrections of drift that had already
shipped: an instrument count that said 14 where the table listed 16 and the code held 21, error
codes documented in lowercase that the handler emits in uppercase, `/readyz` wired and absent from
the OpenAPI file, and nine environment variables read by `internal/config/config.go` that
`.env.example` never named. Every one of those is a copy that fell behind its source.
