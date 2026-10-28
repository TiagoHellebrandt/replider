DROP INDEX IF EXISTS idx_accounts_winback_due;
ALTER TABLE accounts
    DROP COLUMN IF EXISTS winback_started_at,
    DROP COLUMN IF EXISTS winback_coupon_code,
    DROP COLUMN IF EXISTS winback_stage_sent;
