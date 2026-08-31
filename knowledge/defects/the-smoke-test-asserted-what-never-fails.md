---
type: Defect
resource: api/community-waste.postman_collection.json
title: The Newman smoke test asserted a field that every response carries
description: All 17 envelope checks asserted success:true even on 4xx, so a run where about ten requests failed still passed. The failure they hid was folder order — Delete Household ran before the folders that needed the household.
tags: [postman, newman, e2e, ci]
generated: { by: claude/opus-5, at: 2026-08-31T12:00:00Z }
---

# Symptom

The E2E job's Newman run went red only after the assertions were corrected. Before that it was
green while about ten requests answered 400 or 404.

# Two root causes, and the order matters

The reported failure was the ordering one. **Delete Household** lived in the `Households` folder,
and Newman runs folders in file order, so the household was gone before `Waste Pickups`, `Payments`
and `Reports` asked for its id. Those three folders then exercised error paths only. The fix moved
the request into a fifth folder, `Cleanup`, appended after `Reports`.

The reason nobody saw it for as long as they did is the second cause. The 17 assertions named
`Response envelope has success field` and asserted `success: true` with no regard for the status
code. The API answers `success: false` on 4xx, so the assertion was true on the success path and
the only way it could fail was a response with no envelope at all. It graded the shape of a reply
and called it a smoke test.

# The rule this leaves

An assertion that holds for both the passing and the failing case is not a check. Both halves of
the envelope are asserted now: `success: true` on 2xx and `success: false` on 4xx and above.

# What now depends on folder count

Four verifiers carry the number five: `scripts/lint_collections.py`, `scripts/verify_postman_load.js`,
`scripts/verify_insomnia_load.js`, and the Insomnia collection's request groups. A sixth folder is
a four-file change, and the CI contract job is what reports a missed one.
