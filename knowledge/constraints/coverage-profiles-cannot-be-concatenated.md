---
type: Constraint
resource: .github/workflows/ci.yml
title: The coverage gate reads the unit profile alone, because two profiles cannot be concatenated
description: "A profile starts with a mode: header, so cat of two files gives go tool cover two headers and it fails. The failure is silent inside a pipeline, TOTAL comes out empty, and the awk comparison then errors or passes on nothing."
tags: [coverage, ci, gate, makefile]
generated: { by: claude/opus-5, at: 2026-08-31T12:00:00Z }
---

# Why the gate ignores an artifact it downloads

The `coverage-gate` job downloads both `coverage.out` and `coverage-integration.out` and enforces
the threshold against the unit profile only. That is deliberate.

`cat coverage.out coverage-integration.out | go tool cover -func=/dev/stdin` does not merge
anything. Each profile opens with its own `mode:` line, the second one is not a valid record, and
the parse fails. In a pipeline that failure produces no exit code a reader sees. `TOTAL` is then
empty, and `awk "BEGIN { exit ($TOTAL > $MIN ? 0 : 1) }"` is handed a malformed expression.

# The second reason, which outlives the first

The two jobs do not use the same `-coverpkg` list. The unit job excludes `/repository`; the
integration job includes it, because integration tests are what exercise it. A merged number would
be measured over a moving denominator, and it could not be compared against a fixed threshold.

So even with a real merge tool the gate would still be a unit-only gate.

# Before touching this

The comparison is `>`, not `>=`, and `MIN` is a bare number in the workflow. An earlier form of
this gate inverted the test and passed while coverage was below the floor. Any change here has to
be checked with a profile that is known to be under the threshold, not only one that is over it.
