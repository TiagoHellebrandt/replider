-- Relationship routines (§6.5, F4).
--
-- The cheapest revenue in B2B is the customer you already have, and the
-- cheapest way to lose them is silence.
CREATE TABLE IF NOT EXISTS relationship_dates (
    id         CHAR(26)    PRIMARY KEY,
    account_id CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    kind       TEXT        NOT NULL
               CHECK (kind IN ('birthday','founded','sector','maintenance','custom')),
    -- NULL for an account-wide date: a sector day applies to the whole
    -- portfolio, and duplicating it per contact would be six hundred rows
    -- saying the same thing.
    contact_id TEXT        REFERENCES contacts(id) ON DELETE CASCADE,

    -- A RECURRING date. No year on purpose: what matters about a birthday is
    -- that it comes round, and storing 1966 would fire it once.
    month      INTEGER     NOT NULL DEFAULT 0 CHECK (month BETWEEN 0 AND 12),
    day        INTEGER     NOT NULL DEFAULT 0 CHECK (day BETWEEN 0 AND 31),
    -- A ONE-OFF date (maintenance). Mutually exclusive with month/day.
    due_at     TIMESTAMPTZ,

    label      TEXT        NOT NULL DEFAULT '',
    notes      TEXT        NOT NULL DEFAULT '',

    -- Stops a recurring date firing twice in a year: a worker ticking daily
    -- would otherwise greet somebody every day of the lead-time window.
    last_fired_year INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One or the other, never neither: a row with no date is a row the
    -- worker can never act on.
    CONSTRAINT relationship_date_has_a_date
      CHECK ((due_at IS NOT NULL) OR (month > 0 AND day > 0))
);

CREATE INDEX IF NOT EXISTS idx_relationship_dates_account
    ON relationship_dates (account_id);

-- What the worker scans: recurring dates by calendar position, one-offs by
-- their date.
CREATE INDEX IF NOT EXISTS idx_relationship_dates_recurring
    ON relationship_dates (month, day) WHERE due_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_relationship_dates_due
    ON relationship_dates (due_at) WHERE due_at IS NOT NULL;

-- Check-ins already offered, so a customer who stays quiet is not messaged
-- every single day they stay quiet.
CREATE TABLE IF NOT EXISTS check_in_nudges (
    id         CHAR(26)    PRIMARY KEY,
    account_id CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    contact_id TEXT        NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_check_in_nudges_lookup
    ON check_in_nudges (account_id, contact_id, created_at DESC);
