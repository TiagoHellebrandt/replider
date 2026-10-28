-- Pre-billing notification (US-124). We cache the Stripe subscription's
-- current_period_end on the account so a cron can email the admin ~72h
-- before the next charge (reduces involuntary churn / surprise renewals).
-- Synced from customer.subscription.{created,updated} webhooks.
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS current_period_end TIMESTAMPTZ;
-- Dedupe guard: stores the period_end value we last emailed about so the
-- cron sends exactly one reminder per billing cycle even though it ticks
-- repeatedly inside the 72h window.
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS prebilling_notified_period_end TIMESTAMPTZ;

-- Partial index keeps the cron's "who's due?" scan cheap as the table grows.
CREATE INDEX IF NOT EXISTS idx_accounts_current_period_end
    ON accounts (current_period_end)
    WHERE current_period_end IS NOT NULL;
