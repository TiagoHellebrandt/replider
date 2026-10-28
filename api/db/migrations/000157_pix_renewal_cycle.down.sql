-- ALLOW-DESTRUCTIVE: drops the columns this migration added.
DROP INDEX IF EXISTS idx_accounts_pix_cycle_due;
ALTER TABLE accounts
    DROP COLUMN IF EXISTS pix_cycle_stage_sent,
    DROP COLUMN IF EXISTS pix_cycle_period_end,
    DROP COLUMN IF EXISTS pix_cycle_charge_id;
