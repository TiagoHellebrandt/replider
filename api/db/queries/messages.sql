-- name: CreateMessage :execrows
-- ON CONFLICT closes the race between two Meta retries landing within
-- milliseconds of each other (read-then-insert dedupe couldn't catch
-- it). RETURN ROWS = 0 means the row was already persisted by an
-- earlier delivery and the caller should skip downstream side effects
-- (AI run, websocket fanout, billing).
INSERT INTO messages (id, channel_id, conversation_id, sender, sender_name, type, content, media_url, metadata, direction, external_message_id, input_tokens, output_tokens, provider_cost_usd, token_estimated, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NOW())
ON CONFLICT (channel_id, external_message_id)
WHERE external_message_id IS NOT NULL
DO NOTHING;

-- name: GetMessageByExternalID :one
-- Dedupe hook for webhook retries: before inserting an inbound message we
-- check whether the same (channel_id, external_message_id) pair already
-- exists. Meta retries the same wamid when our handler takes too long or
-- returns non-2xx, so this is called on every WhatsApp inbound.
SELECT id FROM messages
WHERE channel_id = $1 AND external_message_id = $2
LIMIT 1;

-- name: GetMessageForAccount :one
-- Loads a single message and enforces account ownership by joining on
-- conversations. Used by the on-demand transcription endpoint — we
-- accept just a message_id from the client and must reject cross-tenant
-- lookups without leaking whether the ID existed.
SELECT m.*
FROM messages m
JOIN conversations c ON c.id = m.conversation_id
WHERE m.id = $1 AND c.account_id = $2
LIMIT 1;

-- name: SetMessageTranscript :exec
-- Caches Whisper/Gemini output on the message row so subsequent opens
-- of the same conversation (or a page refresh) skip the 5-10s API call.
UPDATE messages
SET transcript = $2
WHERE id = $1;

-- name: ApplyWhatsAppStatus :exec
-- Records a Cloud API status update on the matching outbound message.
-- Status timestamps are written conditionally (COALESCE) so a later
-- "delivered" doesn't clear an earlier "read", and the conversation /
-- pricing envelope is only stored on the first status that carries it
-- (Meta only attaches it once per 24h window).
UPDATE messages
SET
    -- Each "*_at" timestamp is set conditionally with COALESCE so an
    -- earlier status (sent → delivered → read → failed) cannot be
    -- clobbered by a later one arriving out of order. failed_at is
    -- handled the same way so a duplicate-failed retry doesn't move
    -- the timestamp forward.
    sent_at             = CASE WHEN sqlc.arg(sent_at)::timestamptz IS NOT NULL
                               THEN COALESCE(sent_at, sqlc.arg(sent_at)::timestamptz)
                               ELSE sent_at END,
    delivered_at        = CASE WHEN sqlc.arg(delivered_at)::timestamptz IS NOT NULL
                               THEN COALESCE(delivered_at, sqlc.arg(delivered_at)::timestamptz)
                               ELSE delivered_at END,
    read_at             = CASE WHEN sqlc.arg(read_at)::timestamptz IS NOT NULL
                               THEN COALESCE(read_at, sqlc.arg(read_at)::timestamptz)
                               ELSE read_at END,
    failed_at           = CASE WHEN sqlc.arg(failed_at)::timestamptz IS NOT NULL
                               THEN COALESCE(failed_at, sqlc.arg(failed_at)::timestamptz)
                               ELSE failed_at END,
    -- NULLIF guards against an empty failed_reason (the Go param is a
    -- non-pointer string so it's always non-NULL) clobbering a real
    -- failure reason set by an earlier status. Status webhooks arrive
    -- out of order — `delivered` after `failed` would otherwise wipe
    -- the reason and the dashboard would show "failed" with no detail.
    failed_reason       = COALESCE(NULLIF(sqlc.arg(failed_reason)::text, ''), failed_reason),
    wa_conversation_id  = COALESCE(wa_conversation_id, NULLIF(sqlc.arg(wa_conversation_id)::text, '')),
    wa_conversation_expires_at = COALESCE(wa_conversation_expires_at, sqlc.arg(wa_conversation_expires_at)::timestamptz),
    conversation_origin = COALESCE(conversation_origin, NULLIF(sqlc.arg(conversation_origin)::text, '')),
    pricing_category    = COALESCE(pricing_category, NULLIF(sqlc.arg(pricing_category)::text, '')),
    pricing_billable    = COALESCE(pricing_billable, sqlc.arg(pricing_billable)::boolean)
WHERE channel_id = sqlc.arg(channel_id) AND external_message_id = sqlc.arg(external_message_id);

-- name: ListMessagesByConversation :many
SELECT * FROM messages
WHERE conversation_id = $1
ORDER BY created_at ASC
LIMIT $2 OFFSET $3;

-- name: ListMessagesByConversationBefore :many
-- Load-more for the transcript: the page of messages OLDER than the
-- cursor, newest first (the caller flips it back to chronological). A
-- cursor rather than OFFSET because new messages keep landing while the
-- attendant scrolls up, and an offset would then repeat or skip rows.
SELECT * FROM messages
WHERE conversation_id = $1 AND created_at < $2
ORDER BY created_at DESC
LIMIT $3;

-- name: ListMessagesByConversationSince :many
SELECT * FROM messages
WHERE conversation_id = $1 AND created_at > $2
ORDER BY created_at ASC
LIMIT 100;

-- name: ListConversationMedia :many
-- Returns every message in the conversation that carries an asset,
-- newest first: a media_url (image / video / audio / document from the
-- chat channels) OR stored attachments (email files live in
-- message_attachments and have media_url NULL — filtering on media_url
-- alone rendered the Arquivos tab empty for every email thread).
SELECT * FROM messages
WHERE conversation_id = $1
  AND ((media_url IS NOT NULL AND media_url <> '')
       OR EXISTS (SELECT 1 FROM message_attachments a
                   WHERE a.message_id = messages.id))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: MarkMessageSendFailed :exec
-- Outbound send failed at the SDK/HTTP layer (distinct from a delivery
-- failure reported later by a status webhook). Stamps failed_at once
-- (COALESCE avoids clobbering an earlier failure) and records the
-- failure reason so the dashboard can show "send failed" rather than
-- silently leaving the bubble as sent.
UPDATE messages
SET failed_at     = COALESCE(failed_at, NOW()),
    failed_reason = COALESCE(NULLIF(failed_reason, ''), $2)
WHERE id = $1;

-- name: GetLastInboundMessageByConversation :one
-- The customer's latest email in the thread: a human reply threads onto its
-- Message-ID / References (otherwise Outlook shows the answer as a loose
-- message outside the conversation) and goes to its Reply-To when present.
SELECT * FROM messages
WHERE conversation_id = $1 AND direction = 'inbound'
ORDER BY created_at DESC
LIMIT 1;
