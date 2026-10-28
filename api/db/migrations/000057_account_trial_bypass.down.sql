DROP INDEX IF EXISTS idx_accounts_trial_bypass;
ALTER TABLE accounts
    DROP COLUMN IF EXISTS trial_bypass_reason,
    DROP COLUMN IF EXISTS trial_bypass;
