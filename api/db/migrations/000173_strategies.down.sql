DROP INDEX IF EXISTS idx_leads_strategy;
ALTER TABLE leads
    DROP COLUMN IF EXISTS strategy_id,
    DROP COLUMN IF EXISTS strategy_version;
DROP TABLE IF EXISTS strategies;
