DROP INDEX IF EXISTS idx_accounts_current_period_end;
ALTER TABLE accounts DROP COLUMN IF EXISTS prebilling_notified_period_end;
ALTER TABLE accounts DROP COLUMN IF EXISTS current_period_end;
