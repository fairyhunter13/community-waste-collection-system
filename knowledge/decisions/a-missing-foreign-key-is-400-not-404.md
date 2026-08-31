---
type: Decision
resource: internal/handler/handler.go
title: A missing foreign key answers 400 on create and 404 on a path id
description: A create body naming an absent household or pickup is rejected by a DB-backed struct validator, so it is 400. The repository still maps the 23503 violation to ErrNotFound, which is 404, and only a row deleted between the two can reach it.
tags: [validation, http-status, postgres, handler, repository]
generated: { by: claude/opus-5, at: 2026-08-31T12:00:00Z }
---

# The split

Two different things can be missing, and they answer differently.

An id in the **path** is looked up by the service, which returns `ErrNotFound`. That is 404.

An id in a **request body** names a row the new record will reference. `CreatePickupRequest` and
`CreatePaymentRequest` tag those fields `db_exists_household` and `db_exists_pickup`, and
`newValidator` registers both against the live pool. A body naming an absent row never reaches the
service, so it is a validation failure, and that is 400.

The rule is the position of the id, not whether the row exists.

# Two layers guard the same violation, and they disagree

`mapPickupCreateErr` and `mapPaymentCreateErr` still map PostgreSQL `23503` to `domain.ErrNotFound`.
That path is not dead and it is not redundant: the validator runs its `SELECT COUNT(*)` in a
separate statement from the insert, so a row deleted in between still reaches the constraint.

The disagreement is the point. The common case is 400 from the validator; the race answers 404 from
the repository. Do not delete the repository mapping to make the two agree, and do not relax the
validator to let the constraint do the work — that would move every bad create to 404 and lose the
distinction above.

# Adding a field that references another table

Tag it `db_exists_X`, then register a matching `RegisterValidationCtx` in `newValidator`. A tag with
no registered validator makes the whole struct fail to parse, which is why the DB validators are
registered even when there is no pool.
