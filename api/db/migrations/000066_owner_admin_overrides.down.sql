DROP INDEX IF EXISTS idx_accounts_suspended;

ALTER TABLE accounts
    DROP COLUMN IF EXISTS suspended_reason,
    DROP COLUMN IF EXISTS suspended_at;
