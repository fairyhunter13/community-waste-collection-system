-- T29 — Composite indexes covering hot query paths.
--
-- payments(household_id, status):
--   General companion to BR-01 / HouseholdHistory. The partial unique
--   uq_payments_one_pending_per_household already covers status='pending',
--   but List / report queries filter on (household_id, status<>pending),
--   which fall back to a single-column scan today.
--
-- payments(status, payment_date):
--   GET /api/payments filters frequently by status combined with a
--   payment_date range. Without this index the planner picks the smaller
--   of idx_payments_status / idx_payments_payment_date and rechecks the
--   other predicate row-by-row.
--
-- waste_pickups(household_id, status):
--   HouseholdHistory + List by household + status filter. Single-column
--   idx_pickups_household_id forces a recheck on status. The organic
--   cancel worker already has the partial idx_pickups_organic_pending_created,
--   so no extra index is needed for that hot path.
CREATE INDEX IF NOT EXISTS idx_payments_household_status
    ON payments(household_id, status);

CREATE INDEX IF NOT EXISTS idx_payments_status_payment_date
    ON payments(status, payment_date)
    WHERE payment_date IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_pickups_household_status
    ON waste_pickups(household_id, status);
