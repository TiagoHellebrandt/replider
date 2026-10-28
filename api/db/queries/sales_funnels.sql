-- name: CreateSalesFunnel :exec
INSERT INTO sales_funnels (
    id, account_id, name, product_id, agent_id, channel_id, persona_prompt,
    metadata_schema, webhook_url, webhook_secret,
    trigger_on_required_complete, trigger_on_buy_intent, active, is_default,
    created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16);

-- name: GetSalesFunnelByID :one
SELECT * FROM sales_funnels WHERE id = $1 LIMIT 1;

-- name: GetSalesFunnelByChannelID :one
-- Resolves the active sales funnel bound to a channel (US-011). A channel
-- backs at most one active funnel for invisible lead-data collection; when
-- several match we take the most recently created.
SELECT * FROM sales_funnels
WHERE channel_id = sqlc.arg('channel_id') AND active = true
ORDER BY created_at DESC
LIMIT 1;

-- name: ListSalesFunnelsByAccountID :many
SELECT * FROM sales_funnels WHERE account_id = $1 ORDER BY created_at DESC;

-- name: UpdateSalesFunnel :exec
UPDATE sales_funnels
SET name = $2,
    product_id = $3,
    agent_id = $4,
    channel_id = $5,
    persona_prompt = $6,
    metadata_schema = $7,
    webhook_url = $8,
    webhook_secret = $9,
    trigger_on_required_complete = $10,
    trigger_on_buy_intent = $11,
    active = $12,
    updated_at = NOW()
WHERE id = $1;

-- name: DeleteSalesFunnel :exec
DELETE FROM sales_funnels WHERE id = $1;

-- name: GetDefaultSalesFunnel :one
-- The account's implicit funnel — where imports and finds land when nobody
-- picked one. At most one row per account (partial unique index).
SELECT * FROM sales_funnels
WHERE account_id = sqlc.arg('account_id') AND is_default
LIMIT 1;

-- name: EnsureDefaultSalesFunnel :one
-- Create-if-missing. Two concurrent callers can both pass the NOT EXISTS
-- and race to the insert; the partial unique index makes the loser fail
-- with a unique violation instead of leaving two defaults, and the
-- repository treats that failure as "someone else just made it" and reads
-- it back. Returns no row when the default already existed.
INSERT INTO sales_funnels (
    id, account_id, name, persona_prompt, metadata_schema,
    webhook_url, webhook_secret,
    trigger_on_required_complete, trigger_on_buy_intent, active, is_default,
    created_at, updated_at
)
SELECT sqlc.arg('id'), sqlc.arg('account_id'), sqlc.arg('name'), '', '[]'::jsonb,
       '', '', FALSE, FALSE, TRUE, TRUE, NOW(), NOW()
WHERE NOT EXISTS (
    SELECT 1 FROM sales_funnels WHERE account_id = sqlc.arg('account_id') AND is_default
)
RETURNING *;
