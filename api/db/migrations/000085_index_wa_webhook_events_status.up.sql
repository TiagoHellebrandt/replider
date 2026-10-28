-- Ops triage feed index for the admin outbox view (US-059).
--
-- `ListWAWebhookEventsByStatus` powers the admin dashboard / runbook
-- commands: `WHERE status = $1 ORDER BY received_at DESC LIMIT $N`.
-- The existing indexes are partial (`pending` on received_at, `processed`
-- on processed_at) and don't cover triage of `dead`/`skipped`/`failed`
-- rows, so that view falls back to a full scan + sort as the table grows.
--
-- A composite (status, received_at DESC) index serves both the equality
-- filter and the ordering in one range scan for any status value, keeping
-- the admin endpoint well under 500ms.
CREATE INDEX idx_wa_webhook_events_status_received_at
    ON wa_webhook_events(status, received_at DESC);
