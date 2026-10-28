-- Operational analytics: stamp the "first inbound" and "first response
-- by whom" markers so we can compute total conversations, % resolved
-- by AI alone, and avg first-response time without scanning the full
-- messages history per account.
--
-- Scope note: first_response_at already exists (migration 000008);
-- CSAT data lives in csat_ratings (migration 000024) — both reused
-- as-is. Backfill intentionally skipped; the dashboard documents
-- that "numbers start counting from the rollout date".

ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS first_inbound_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS first_response_by TEXT NULL
        CHECK (first_response_by IN ('bot','human'));

-- Supports the "% resolved by AI" aggregate: we filter by
-- first_response_by AND is_bot_paused=false so the query hits only
-- the columns it needs without scanning the whole row.
CREATE INDEX IF NOT EXISTS idx_conversations_first_response_by
    ON conversations(account_id, first_response_by)
    WHERE first_response_by IS NOT NULL;
