DROP INDEX IF EXISTS idx_accounts_reactivation_due;

ALTER TABLE accounts
    DROP COLUMN IF EXISTS reactivation_stage_sent;
