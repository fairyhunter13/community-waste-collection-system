-- T27 — Partial UNIQUE index enforcing at most one pending payment per
-- household at the DB tier. This is the schema-level companion to BR-01 and the
-- BR-05 transactional check: even if two parallel Complete callers slip past
-- the SELECT FOR UPDATE / conditional UPDATE in application code, the second
-- payment INSERT will hit this constraint and fail with 23505 unique_violation,
-- which the repository maps to ErrConflict.
CREATE UNIQUE INDEX IF NOT EXISTS uq_payments_one_pending_per_household
    ON payments(household_id)
    WHERE status = 'pending';
