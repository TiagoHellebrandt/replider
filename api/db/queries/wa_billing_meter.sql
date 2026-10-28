-- name: AggregateWAConversationsForMetering :exec
-- Materializes new wa_billing_meter rows for every distinct
-- (account, wa_conversation_id, pricing_category) tuple seen on the
-- messages table for the given billing period. ON CONFLICT DO
-- NOTHING means re-running the aggregator (daily tick + manual
-- backfill) is idempotent: existing rows keep their metered_at and
-- a new row only lands when a brand-new conversation crosses the
-- threshold.
INSERT INTO wa_billing_meter (
    id, account_id, wa_conversation_id, pricing_category,
    conversation_origin, pricing_billable, billing_period
)
SELECT
    -- ULID generated client-side would be cleaner; we use
    -- gen_random_uuid()::char(26) here as a one-shot ID since the
    -- value is never surfaced externally. The unique index is the
    -- real dedupe key.
    substring(replace(gen_random_uuid()::text, '-', '') from 1 for 26),
    c.account_id,
    m.wa_conversation_id,
    m.pricing_category,
    MAX(m.conversation_origin),
    bool_or(COALESCE(m.pricing_billable, TRUE)),
    date_trunc('month', MIN(m.created_at))::date
FROM messages m
JOIN conversations c ON c.id = m.conversation_id
WHERE m.wa_conversation_id IS NOT NULL
  AND m.pricing_category IS NOT NULL
  AND m.created_at >= sqlc.arg(period_start)::timestamptz
  AND m.created_at < sqlc.arg(period_end)::timestamptz
GROUP BY c.account_id, m.wa_conversation_id, m.pricing_category
ON CONFLICT (account_id, wa_conversation_id, pricing_category) DO NOTHING;

-- name: ListPendingMeterRollups :many
-- Per-account aggregated counts ready to push to Stripe. Joined
-- against plans so the worker has the right subscription_item_id
-- for the category. Only rows where metered_at IS NULL contribute.
SELECT
    m.account_id,
    m.pricing_category,
    m.billing_period,
    COUNT(*) FILTER (WHERE m.pricing_billable)::int AS billable_count,
    COUNT(*)::int AS total_count,
    p.stripe_marketing_item_id,
    p.stripe_utility_item_id,
    p.stripe_authentication_item_id,
    p.stripe_service_item_id
FROM wa_billing_meter m
JOIN accounts a ON a.id = m.account_id
LEFT JOIN plans p ON p.id = a.plan_id
WHERE m.metered_at IS NULL
GROUP BY m.account_id, m.pricing_category, m.billing_period,
         p.stripe_marketing_item_id, p.stripe_utility_item_id,
         p.stripe_authentication_item_id, p.stripe_service_item_id
ORDER BY m.billing_period ASC, m.account_id ASC
LIMIT sqlc.arg(batch_size)::int;

-- name: MarkMeterRollupPushed :execrows
-- Stamp metered_at on every row matching the (account, category,
-- period) tuple after the Stripe push succeeded. Idempotent:
-- already-stamped rows aren't touched.
UPDATE wa_billing_meter
SET metered_at = NOW()
WHERE account_id = sqlc.arg(account_id)::char(26)
  AND pricing_category = sqlc.arg(pricing_category)::text
  AND billing_period = sqlc.arg(billing_period)::date
  AND metered_at IS NULL;
