CREATE TABLE IF NOT EXISTS households (
    id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_name  TEXT          NOT NULL CHECK (char_length(owner_name) >= 1),
    address     TEXT          NOT NULL CHECK (char_length(address) >= 1),
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);
