-- Outbound message scheduler. Stages a future message + dispatch
-- payload so the campaign / agent layer can queue "send tomorrow at
-- 10am" without spawning long-lived goroutines. The dispatcher
-- worker drains rows whose send_at has fallen due, sends them via
-- the channel adapter, and marks the row processed.
--
-- Status transitions:
--   pending  -> sent      (success)
--   pending  -> failed    (provider error after max retries)
--   pending  -> cancelled (manual ops abort)
--
-- Why a separate table instead of overloading messages: the message
-- row exists once we KNOW it shipped. Until send_at fires, only the
-- intent + payload exist. Coupling the two would force us to write
-- "scheduled but not sent" rows into messages, polluting the
-- conversation transcript.
CREATE TABLE outbound_scheduled (
    id              CHAR(26) PRIMARY KEY,
    account_id      CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    channel_id      CHAR(26) NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    conversation_id CHAR(26) REFERENCES conversations(id) ON DELETE SET NULL,
    -- payload carries the full domain.Message JSON (type, content,
    -- media_url, metadata). Stored opaque so a future field doesn't
    -- require a migration.
    payload         JSONB NOT NULL,
    recipient       TEXT NOT NULL,
    send_at         TIMESTAMPTZ NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    attempts        INT NOT NULL DEFAULT 0,
    last_error      TEXT,
    sent_message_id CHAR(26) REFERENCES messages(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Drain index: ordered by send_at so the worker pulls the oldest
-- due rows first. Partial because the worker only ever scans pending.
CREATE INDEX idx_outbound_scheduled_due
    ON outbound_scheduled(send_at)
    WHERE status = 'pending';

-- Per-account list index for the dashboard's "scheduled messages"
-- view.
CREATE INDEX idx_outbound_scheduled_account
    ON outbound_scheduled(account_id, send_at);
