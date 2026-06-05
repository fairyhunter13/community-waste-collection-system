CREATE TYPE payment_status AS ENUM ('pending', 'paid', 'failed');

CREATE TABLE IF NOT EXISTS payments (
    id             UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id   UUID           NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    waste_id       UUID           NOT NULL UNIQUE REFERENCES waste_pickups(id) ON DELETE CASCADE,
    amount         NUMERIC(12,2)  NOT NULL CHECK (amount > 0),
    payment_date   TIMESTAMPTZ,
    status         payment_status NOT NULL DEFAULT 'pending',
    proof_file_url TEXT,
    created_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_household_id ON payments(household_id);
CREATE INDEX idx_payments_status       ON payments(status);

-- Partial index; only paid payments have a payment_date
CREATE INDEX idx_payments_payment_date ON payments(payment_date)
    WHERE payment_date IS NOT NULL;
