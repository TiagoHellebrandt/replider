-- US-178: In-app NPS prompt (quarterly). Stores one row per submitted
-- Net Promoter Score response so the admin dashboard can compute NPS and
-- read recent verbatim comments. The quarterly "show once" gate is a
-- client-side concern; this table is the durable record of what was sent.
CREATE TABLE IF NOT EXISTS nps_responses (
    id         TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    user_id    TEXT NOT NULL,
    score      INTEGER NOT NULL CHECK (score >= 0 AND score <= 10),
    comment    TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Window scan for the admin NPS aggregation (count by score bucket over a
-- trailing period).
CREATE INDEX IF NOT EXISTS idx_nps_responses_created ON nps_responses (created_at);

-- Per-account / per-user lookup (admin drill-down + the optional
-- already-responded-this-quarter server guard).
CREATE INDEX IF NOT EXISTS idx_nps_responses_account_created ON nps_responses (account_id, created_at);
