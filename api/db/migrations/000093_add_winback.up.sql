-- US-179: Win-back flow on cancellation.
-- When a paid subscription is canceled we mint an automatic
-- 50%-off-3-months recovery coupon and drip follow-up reminders at 7/30
-- days. These columns drive that flow:
--   winback_started_at  anchors the day counter (set when the subscription
--                       is deleted; NULL = not in the flow);
--   winback_coupon_code carries the personal promo code into every email;
--   winback_stage_sent  is the last follow-up milestone emailed (0 = only
--                       the initial offer went out) so the daily worker is
--                       idempotent and never re-sends a stage.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS winback_started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS winback_coupon_code TEXT,
    ADD COLUMN IF NOT EXISTS winback_stage_sent SMALLINT NOT NULL DEFAULT 0;

-- Partial index over the small set of recently-churned, not-yet-fully-nudged
-- accounts the follow-up worker scans. Matches the WHERE in ListWinBackDue.
CREATE INDEX IF NOT EXISTS idx_accounts_winback_due
    ON accounts (winback_started_at)
    WHERE winback_started_at IS NOT NULL
      AND winback_stage_sent < 30;
