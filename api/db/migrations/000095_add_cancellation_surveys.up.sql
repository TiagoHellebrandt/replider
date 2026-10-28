-- US-180: Cancellation reason survey. Stores one row per submitted
-- cancellation survey so the admin dashboard can break down churn reasons.
-- The survey is presented on the Stripe Portal cancel return URL (a custom
-- form in the app); this table is the durable record of why customers leave.
CREATE TABLE IF NOT EXISTS cancellation_surveys (
    id         TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    user_id    TEXT NOT NULL,
    reason     TEXT NOT NULL,
    comment    TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Window scan for the admin breakdown aggregation (count by reason over a
-- trailing period).
CREATE INDEX IF NOT EXISTS idx_cancellation_surveys_created ON cancellation_surveys (created_at);

-- Per-account drill-down.
CREATE INDEX IF NOT EXISTS idx_cancellation_surveys_account_created ON cancellation_surveys (account_id, created_at);
