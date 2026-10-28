DROP TABLE IF EXISTS economic_index_values;

ALTER TABLE contracts
    DROP COLUMN IF EXISTS price_registry,
    DROP COLUMN IF EXISTS registry_number,
    DROP COLUMN IF EXISTS registry_balance_cents,
    DROP COLUMN IF EXISTS registry_balance_on,
    DROP COLUMN IF EXISTS last_adjusted_on;
