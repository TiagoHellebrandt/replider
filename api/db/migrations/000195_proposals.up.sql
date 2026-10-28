-- A priced offer made to one customer at one moment (pillar 3).
--
-- Separate from the lead's `approach_draft` on purpose: that column is a
-- scratchpad the next draft overwrites, and "more good offers at the right time"
-- needs the history of what was already offered.
CREATE TABLE IF NOT EXISTS proposals (
    id          TEXT PRIMARY KEY,
    account_id  TEXT NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    -- The lead is the pipeline card. Deleting it takes its offers: an offer with
    -- no addressee is not a record of anything.
    lead_id     TEXT NOT NULL REFERENCES leads (id) ON DELETE CASCADE,
    status      TEXT NOT NULL DEFAULT 'draft',
    -- Governs every amount below. Never inferred from the reader's locale.
    currency    TEXT NOT NULL,
    body        TEXT NOT NULL DEFAULT '',
    sent_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The list a salesperson opens: this customer's offers, newest first.
CREATE INDEX IF NOT EXISTS idx_proposals_lead
    ON proposals (account_id, lead_id, created_at DESC);

CREATE TABLE IF NOT EXISTS proposal_items (
    id                TEXT PRIMARY KEY,
    proposal_id       TEXT NOT NULL REFERENCES proposals (id) ON DELETE CASCADE,
    -- SET NULL, not CASCADE: discontinuing a product must not erase the line
    -- from an offer somebody already received.
    product_id        TEXT REFERENCES products (id) ON DELETE SET NULL,
    -- Snapshots. The catalogue is free to rename and reprice; an offer already
    -- made is not.
    name              TEXT NOT NULL,
    quantity          INTEGER NOT NULL CHECK (quantity > 0),
    unit_price_cents  BIGINT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proposal_items_proposal
    ON proposal_items (proposal_id);
