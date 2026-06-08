CREATE TYPE waste_type AS ENUM ('organic', 'plastic', 'paper', 'electronic');
CREATE TYPE pickup_status AS ENUM ('pending', 'scheduled', 'completed', 'canceled');

CREATE TABLE IF NOT EXISTS waste_pickups (
    id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID          NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    type         waste_type    NOT NULL,
    status       pickup_status NOT NULL DEFAULT 'pending',
    pickup_date  TIMESTAMPTZ,
    safety_check BOOLEAN       NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pickups_household_id ON waste_pickups(household_id);
CREATE INDEX IF NOT EXISTS idx_pickups_status       ON waste_pickups(status);
CREATE INDEX IF NOT EXISTS idx_pickups_type_status  ON waste_pickups(type, status);

-- Partial index for the organic auto-cancel worker query
CREATE INDEX IF NOT EXISTS idx_pickups_organic_pending_created
    ON waste_pickups(created_at)
    WHERE type = 'organic' AND status = 'pending';
