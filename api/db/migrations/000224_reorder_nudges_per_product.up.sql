-- A nudge is per predicted PRODUCT, not per sale (analysis 2026-09-01 §7):
-- one sale carrying gloves and gauze produces two predictions, and the old
-- UNIQUE(account_id, sale_id) let only the first ever be offered — the
-- second product was silently never asked about.
ALTER TABLE reorder_nudges
    ADD COLUMN IF NOT EXISTS product_key TEXT NOT NULL DEFAULT '';

DROP INDEX IF EXISTS uniq_reorder_nudge;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_reorder_nudge
    ON reorder_nudges (account_id, sale_id, product_key);
