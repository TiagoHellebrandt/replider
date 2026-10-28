-- Webhook outbox for WhatsApp Cloud API.
--
-- Today the inbound handler parses + persists + fans out side effects
-- on the request goroutine. If the parser or DB hiccups mid-flow, the
-- side effects either fire twice (Meta retries the webhook) or fail
-- permanently (Meta gives up after 3 retries within ~1 minute).
--
-- The outbox decouples receipt from processing: every signed payload
-- lands here first, the handler returns 200 immediately, and the
-- async pump drains rows. Failures stay marked `pending` with an
-- attempts counter so a transient AI/quota error gets a second
-- chance without re-triggering Meta retries.
--
-- Status state machine:
--   pending   → row received, not yet processed (or last attempt errored).
--   processed → all side effects committed.
--   skipped   → recognised but intentionally not actionable (e.g. test
--               webhooks from Meta App dashboard).
--   dead      → max attempts exceeded; needs human review.
CREATE TABLE wa_webhook_events (
    id              TEXT PRIMARY KEY,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    payload         BYTEA       NOT NULL,
    signature       TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'pending',
    processed_at    TIMESTAMPTZ,
    error           TEXT,
    attempts        INT         NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ
);

-- Drain index: the worker scans rows in receipt order, oldest pending
-- first, with a status filter to skip processed rows quickly.
CREATE INDEX idx_wa_webhook_events_pending
    ON wa_webhook_events(received_at)
    WHERE status = 'pending';

-- Janitor index: a daily cleanup job purges processed rows older than
-- 14 days. Keeping 14d gives us a debug window for Meta-side replay
-- bugs without unbounded growth.
CREATE INDEX idx_wa_webhook_events_processed_at
    ON wa_webhook_events(processed_at)
    WHERE status = 'processed';
