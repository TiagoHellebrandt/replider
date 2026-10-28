-- Repurchase cycles on the product (§6.5, F3).
--
-- Consumables are recurring revenue. A customer whose repurchase works does
-- not churn; one who has to remember to reorder eventually remembers a
-- competitor instead.
ALTER TABLE products
    -- How long a customer typically takes to go through one order of this.
    -- The fallback when a sale did not record its own cycle — which is most
    -- sales, because nobody types a cycle for a product they sell weekly.
    ADD COLUMN IF NOT EXISTS consumption_cycle_days INTEGER NOT NULL DEFAULT 0,
    -- Shelf life, which is a DIFFERENT number: gauze expires whether or not
    -- anyone uses it, and a reorder offered after the expiry date is a
    -- reorder for stock the customer has to throw away.
    ADD COLUMN IF NOT EXISTS shelf_life_days INTEGER NOT NULL DEFAULT 0;

-- Repurchase approaches already offered, so the worker does not ask the same
-- customer every day for a week.
CREATE TABLE IF NOT EXISTS reorder_nudges (
    id         CHAR(26)    PRIMARY KEY,
    account_id CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    contact_id TEXT        NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    -- The sale that produced the prediction. Keyed with the contact so a
    -- customer who buys two different things still gets two nudges.
    sale_id    CHAR(26)    NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    due_at     TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One nudge per predicted reorder. A worker that runs hourly must not turn a
-- helpful reminder into a week of daily ones.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_reorder_nudge
    ON reorder_nudges (account_id, sale_id);
