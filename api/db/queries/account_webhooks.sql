-- name: CreateAccountWebhook :one
INSERT INTO account_webhooks (id, account_id, url, secret, events, is_active, description)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListAccountWebhooks :many
-- Used by the settings UI. Inactive rows still surface so the admin
-- can re-enable them — auto-disabled webhooks (excessive failures)
-- need to be visible for triage, not hidden.
SELECT * FROM account_webhooks
WHERE account_id = $1
ORDER BY created_at DESC;

-- name: ListActiveWebhooksForEvent :many
-- Hot path for the dispatcher. Combines the partial-active index +
-- ANY-array filter on `events` so a single account with a dozen
-- webhooks but only one subscribing to message.received still
-- returns one row, not all twelve.
SELECT * FROM account_webhooks
WHERE account_id = sqlc.arg(account_id)
  AND is_active = true
  AND sqlc.arg(event_name)::text = ANY(events);

-- name: GetAccountWebhook :one
SELECT * FROM account_webhooks
WHERE id = $1 AND account_id = $2;

-- name: UpdateAccountWebhook :one
UPDATE account_webhooks
SET url = $3, events = $4, is_active = $5, description = $6, updated_at = NOW()
WHERE id = $1 AND account_id = $2
RETURNING *;

-- name: DeleteAccountWebhook :exec
DELETE FROM account_webhooks
WHERE id = $1 AND account_id = $2;

-- name: RecordWebhookSuccess :exec
-- Resets failure counter so a row that was on the edge of auto-
-- disable doesn't get killed by a stale streak.
UPDATE account_webhooks
SET last_fired_at = NOW(),
    last_succeeded_at = NOW(),
    last_error = NULL,
    failure_count = 0
WHERE id = $1;

-- name: RecordWebhookFailure :exec
-- Increments failure_count + stamps the error. Dispatcher decides
-- whether to flip is_active=false after N consecutive failures.
UPDATE account_webhooks
SET last_fired_at = NOW(),
    last_error = $2,
    failure_count = failure_count + 1
WHERE id = $1;

-- name: DisableWebhook :exec
-- Auto-disable trip wire. Manual re-enable goes through Update.
UPDATE account_webhooks
SET is_active = false, updated_at = NOW()
WHERE id = $1;
