-- name: CreateCampaign :exec
INSERT INTO campaigns (
    id, account_id, channel_id, name, template_name, template_lang, template_vars,
    status, scheduled_at, total_recipients, created_by, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW()
);

-- name: GetCampaignByID :one
SELECT * FROM campaigns WHERE id = $1 AND account_id = $2;

-- name: ListCampaignsByAccount :many
SELECT * FROM campaigns
WHERE account_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateCampaignStatus :exec
UPDATE campaigns
SET status = $2,
    started_at = COALESCE(started_at, CASE WHEN $2 = 'sending' THEN NOW() ELSE NULL END),
    finished_at = CASE WHEN $2 IN ('done','failed','canceled') THEN NOW() ELSE finished_at END,
    updated_at = NOW()
WHERE id = $1;

-- name: IncrementCampaignCounter :exec
UPDATE campaigns
SET sent_count      = sent_count      + CASE WHEN $2 = 'sent'      THEN 1 ELSE 0 END,
    delivered_count = delivered_count + CASE WHEN $2 = 'delivered' THEN 1 ELSE 0 END,
    read_count      = read_count      + CASE WHEN $2 = 'read'      THEN 1 ELSE 0 END,
    failed_count    = failed_count    + CASE WHEN $2 = 'failed'    THEN 1 ELSE 0 END,
    updated_at = NOW()
WHERE id = $1;

-- name: DeleteCampaign :exec
DELETE FROM campaigns WHERE id = $1 AND account_id = $2;

-- name: ListScheduledCampaignsDue :many
-- Used by the worker to pick up campaigns that should now be sent.
SELECT * FROM campaigns
WHERE status = 'scheduled'
  AND (scheduled_at IS NULL OR scheduled_at <= NOW())
ORDER BY scheduled_at NULLS FIRST
LIMIT $1;

-- ─── Recipients ─────────────────────────────────────────────────────────────

-- name: CreateRecipient :exec
INSERT INTO campaign_recipients (id, campaign_id, recipient, vars, status, created_at)
VALUES ($1, $2, $3, $4, 'pending', NOW())
ON CONFLICT (campaign_id, recipient) DO NOTHING;

-- name: ListRecipientsByCampaign :many
SELECT * FROM campaign_recipients
WHERE campaign_id = $1
ORDER BY created_at ASC
LIMIT $2 OFFSET $3;

-- name: ListPendingRecipients :many
SELECT * FROM campaign_recipients
WHERE campaign_id = $1 AND status = 'pending'
ORDER BY created_at ASC
LIMIT $2;

-- name: MarkRecipientSent :exec
UPDATE campaign_recipients
SET status = 'sent', sent_at = NOW(), provider_msg_id = $2
WHERE id = $1;

-- name: MarkRecipientFailed :exec
UPDATE campaign_recipients
SET status = 'failed', failed_at = NOW(), error_message = $2
WHERE id = $1;

-- name: UpdateRecipientStatusByProviderID :exec
-- Called by the inbound webhook when WhatsApp reports delivered/read updates.
UPDATE campaign_recipients
SET status      = $2,
    delivered_at = CASE WHEN $2 = 'delivered' THEN NOW() ELSE delivered_at END,
    read_at      = CASE WHEN $2 = 'read'      THEN NOW() ELSE read_at      END
WHERE provider_msg_id = $1;

-- name: GetCampaignByRecipientProviderID :one
SELECT c.* FROM campaigns c
JOIN campaign_recipients r ON r.campaign_id = c.id
WHERE r.provider_msg_id = $1
LIMIT 1;
