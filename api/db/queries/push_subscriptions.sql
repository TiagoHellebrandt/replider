-- Push subscriptions: web/mobile push endpoints per dashboard user.
-- Repository uses direct pgx today (see push_subscription_repository.go);
-- this file is kept in sync so `sqlc generate` remains the source of truth
-- if we migrate the repo to generated code later.

-- name: UpsertPushSubscription :exec
INSERT INTO push_subscriptions (
    id, user_id, account_id, platform, endpoint,
    p256dh_key, auth_key, device_token, user_agent, enabled, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, TRUE, NOW(), NOW())
ON CONFLICT (user_id, endpoint) DO UPDATE
SET p256dh_key   = EXCLUDED.p256dh_key,
    auth_key     = EXCLUDED.auth_key,
    device_token = EXCLUDED.device_token,
    user_agent   = EXCLUDED.user_agent,
    enabled      = TRUE,
    updated_at   = NOW();

-- name: ListPushSubscriptionsByAccount :many
SELECT * FROM push_subscriptions
WHERE account_id = $1 AND enabled = TRUE;

-- name: ListPushSubscriptionsByUser :many
SELECT * FROM push_subscriptions
WHERE user_id = $1 AND enabled = TRUE
ORDER BY created_at DESC;

-- name: DeletePushSubscriptionByEndpoint :exec
DELETE FROM push_subscriptions
WHERE user_id = $1 AND endpoint = $2;

-- name: DisablePushSubscriptionByEndpoint :exec
UPDATE push_subscriptions
SET enabled = FALSE, updated_at = NOW()
WHERE endpoint = $1;

-- name: TouchPushSubscription :exec
UPDATE push_subscriptions
SET last_used_at = NOW()
WHERE id = $1;
