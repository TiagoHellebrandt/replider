ALTER TABLE accounts DROP COLUMN IF EXISTS inbound_debounce_seconds;

DROP INDEX IF EXISTS idx_inbound_debounce_claimed;
DROP INDEX IF EXISTS idx_inbound_debounce_due;
DROP TABLE IF EXISTS inbound_debounce;
