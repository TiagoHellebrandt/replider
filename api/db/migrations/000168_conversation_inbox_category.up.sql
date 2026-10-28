-- Inbox triage (§6.10, camada 3).
--
-- A shared commercial mailbox is not a chat stream: in one morning it holds a
-- lead answering a cold email, a supplier's invoice, three newsletters and a
-- phishing attempt. Chronological order buries the first under the last three.
ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS inbox_category   TEXT,
    -- Higher sorts first. Stored rather than computed at read time so the
    -- ordering happens in SQL, before LIMIT/OFFSET — sorting a page in Go
    -- would reorder 50 rows and still show the wrong 50.
    ADD COLUMN IF NOT EXISTS inbox_importance SMALLINT NOT NULL DEFAULT 0;

-- The inbox filters by category within an account and orders by importance.
CREATE INDEX IF NOT EXISTS idx_conversations_inbox
    ON conversations (account_id, inbox_category)
    WHERE inbox_category IS NOT NULL;
