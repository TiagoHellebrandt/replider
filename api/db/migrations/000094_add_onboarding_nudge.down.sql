DROP INDEX IF EXISTS idx_accounts_onboarding_nudge_due;
ALTER TABLE accounts
    DROP COLUMN IF EXISTS onboarding_nudge_stage_sent;
