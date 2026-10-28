-- name: GetChannelByWebhookToken :one
SELECT * FROM channels
WHERE webhook_token = $1 AND is_active = true LIMIT 1;

-- name: CreateChannel :one
INSERT INTO channels (id, account_id, agent_id, provider, name, credentials, webhook_token, is_active, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, true, NOW(), NOW())
RETURNING *;

-- name: ListChannelsByAccountID :many
SELECT * FROM channels
WHERE account_id = $1
ORDER BY created_at DESC;

-- name: ListChannelsByAgentID :many
SELECT * FROM channels
WHERE agent_id = $1
ORDER BY created_at DESC;

-- name: ListAllChannelsByProvider :many
SELECT * FROM channels
WHERE provider = $1
ORDER BY created_at DESC;

-- name: GetChannelByID :one
SELECT * FROM channels
WHERE id = $1 LIMIT 1;

-- name: UpdateChannel :one
UPDATE channels
SET name = $2, credentials = $3, agent_id = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteChannel :exec
DELETE FROM channels WHERE id = $1;

-- name: ToggleChannel :one
UPDATE channels
SET is_active = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: GetChannelByPhoneNumberID :one
SELECT * FROM channels
WHERE provider = 'whatsapp'
  AND credentials->>'phone_number_id' = sqlc.arg(phone_number_id)::text
  AND is_active = true
LIMIT 1;

-- name: GetChannelByWABAID :one
-- Used by Meta's webhook fan-out: events scoped to a WABA (template
-- approvals, quality drops, account review) target the entry.id which
-- equals the WABA. Active channels only — inactive ones can't act on
-- the event anyway. Returns the first match deterministically when a
-- WABA hosts multiple phone numbers / channels.
SELECT * FROM channels
WHERE provider = 'whatsapp'
  AND credentials->>'waba_id' = sqlc.arg(waba_id)::text
  AND is_active = true
ORDER BY created_at ASC
LIMIT 1;

-- name: ListChannelsByWABAID :many
-- Returns every active WhatsApp channel under a WABA. Used by webhook
-- fan-out for events that should reach EVERY tenant sharing the WABA
-- (template approvals broadcast to all phone numbers under it, quality
-- update may carry display_phone_number that pins a single channel).
-- The single-row GetChannelByWABAID variant silently leaks events to
-- the oldest channel only — if two Replider accounts each registered
-- a different phone under the same WABA, account A receives account
-- B's events and vice-versa.
SELECT * FROM channels
WHERE provider = 'whatsapp'
  AND credentials->>'waba_id' = sqlc.arg(waba_id)::text
  AND is_active = true
ORDER BY created_at ASC;

-- name: UpdateChannelCredentialString :exec
-- Partial JSONB update for a single string-valued credential key. Used
-- by the WhatsApp token-refresh worker so it can rotate access_token
-- without re-marshalling the WHOLE credentials blob — read-modify-write
-- on JSONB races against concurrent writes from the admin handler
-- (registration_status, registration_error_*, subscribe_status) and
-- silently overwrites whichever side wrote last.
--
-- jsonb_set(target, path, new_value, create_if_missing) returns a new
-- JSONB with only the specified key replaced; every other key keeps
-- its current value, so two writers touching different keys no longer
-- step on each other.
UPDATE channels
SET credentials = jsonb_set(
        credentials,
        ARRAY[sqlc.arg(key)::text],
        to_jsonb(sqlc.arg(value)::text),
        true
    ),
    updated_at  = NOW()
WHERE id = sqlc.arg(id)::char(26);

-- name: MergeChannelCredentials :exec
-- Multi-key partial JSONB merge using PostgreSQL's `||` operator: keys
-- in the supplied JSONB override existing values, others stay untouched.
-- Used by the WhatsApp register / verify-code / deregister handlers
-- after they update several status fields at once
-- (registration_status, registration_error_*, subscribe_status,
-- last_pin) — running 6 individual jsonb_set queries would be wasteful
-- and the previous Update() path replaced the WHOLE blob, undoing
-- whatever the token worker had written between read and write.
UPDATE channels
SET credentials = credentials || sqlc.arg(patch)::jsonb,
    updated_at  = NOW()
WHERE id = sqlc.arg(id)::char(26);
