---
type: Constraint
resource: codecov.yml
title: The codecov ignore list has to mirror the COVERPKG filter
description: The Makefile drops internal/observability from -coverpkg, so no coverage data is produced for it at all. Removing it from codecov.yml does not measure it; it reports it as uncovered.
tags: [coverage, codecov, makefile, ci]
generated: { by: claude/opus-5, at: 2026-08-31T12:00:00Z }
---

# The coupling

`make test-unit` builds its `-coverpkg` list with a grep that excludes `/mocks`, `/repository` and
`/observability`. A package outside `-coverpkg` produces no statements in the profile. Codecov
receives nothing for it.

`codecov.yml` decides what Codecov *reports*. A package with no data that is not ignored is not
reported as unmeasured. It is reported as zero.

So the two lists are one decision written in two files, and only the Makefile half has any effect
on what is measured.

# The reversal that proves it

The ignore entry for `internal/observability/**` was removed once, on the reasonable-sounding
argument that first-party hand-written code should be measured alongside the rest. That did not
measure it. It only moved the reported number, because the Makefile filter was untouched. The
entry was restored in the next commit that touched coverage, and the reason is now the comment in
`codecov.yml`.

# Before changing either half

Change the Makefile first. If a package should be measured, take it out of the grep and confirm
that `coverage.out` gains statements for it. Only then take it out of the ignore list. The reverse
order reports a false number and passes.

Generated mocks stay excluded in both places, per Codecov's own guidance.
