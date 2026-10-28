-- The up-migration widened the unique key to (account_id, sale_id, product_key)
-- precisely so one sale can hold several product nudges. Collapsing back to the
-- narrow UNIQUE(account_id, sale_id) would abort on those duplicates, leaving
-- the rollback half-applied — so first keep one row per (account_id, sale_id)
-- (the most recent, by ctid) before recreating the narrow index.
DELETE FROM reorder_nudges a
  USING reorder_nudges b
  WHERE a.account_id = b.account_id
    AND a.sale_id = b.sale_id
    AND a.ctid < b.ctid;
DROP INDEX IF EXISTS uniq_reorder_nudge;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_reorder_nudge
    ON reorder_nudges (account_id, sale_id);
ALTER TABLE reorder_nudges DROP COLUMN IF EXISTS product_key;
