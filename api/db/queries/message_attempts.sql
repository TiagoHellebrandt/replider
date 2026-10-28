-- name: CreateMessageAttempt :exec
-- Records one outbound send attempt for a parent message row. Used by
-- the WhatsApp bot reply pipeline so each bubble in a split AI reply
-- has its own row to correlate later status updates against. Failed
-- sends still get a row (with NULL external_message_id) so ops can
-- count attempts without scanning logs.
INSERT INTO message_attempts (
    id, message_id, channel_id, external_message_id, bubble_index,
    sent_at, failed_at, failed_reason
) VALUES (
    sqlc.arg(id)::char(26),
    sqlc.arg(message_id)::char(26),
    sqlc.arg(channel_id)::char(26),
    NULLIF(sqlc.arg(external_message_id)::text, ''),
    sqlc.arg(bubble_index)::smallint,
    sqlc.arg(sent_at)::timestamptz,
    sqlc.arg(failed_at)::timestamptz,
    NULLIF(sqlc.arg(failed_reason)::text, '')
);

-- name: ApplyMessageAttemptStatus :exec
-- Mirrors ApplyWhatsAppStatus on the messages table — same COALESCE
-- + NULLIF guards so an out-of-order delivered/read/failed sequence
-- can't clobber an earlier-stamped column.
UPDATE message_attempts
SET
    sent_at      = CASE WHEN sqlc.arg(sent_at)::timestamptz IS NOT NULL
                        THEN COALESCE(sent_at, sqlc.arg(sent_at)::timestamptz)
                        ELSE sent_at END,
    delivered_at = CASE WHEN sqlc.arg(delivered_at)::timestamptz IS NOT NULL
                        THEN COALESCE(delivered_at, sqlc.arg(delivered_at)::timestamptz)
                        ELSE delivered_at END,
    read_at      = CASE WHEN sqlc.arg(read_at)::timestamptz IS NOT NULL
                        THEN COALESCE(read_at, sqlc.arg(read_at)::timestamptz)
                        ELSE read_at END,
    failed_at    = CASE WHEN sqlc.arg(failed_at)::timestamptz IS NOT NULL
                        THEN COALESCE(failed_at, sqlc.arg(failed_at)::timestamptz)
                        ELSE failed_at END,
    failed_reason = COALESCE(NULLIF(sqlc.arg(failed_reason)::text, ''), failed_reason),
    updated_at   = NOW()
WHERE channel_id = sqlc.arg(channel_id)::char(26)
  AND external_message_id = sqlc.arg(external_message_id)::text;

-- name: GetMessageAttemptByExternalID :one
-- Used by the inbound status webhook to resolve a wamid back to its
-- parent message + bubble index for dashboard rendering.
SELECT id, message_id, channel_id, external_message_id, bubble_index,
       sent_at, delivered_at, read_at, failed_at, failed_reason,
       created_at, updated_at
FROM message_attempts
WHERE channel_id = sqlc.arg(channel_id)::char(26)
  AND external_message_id = sqlc.arg(external_message_id)::text
LIMIT 1;
