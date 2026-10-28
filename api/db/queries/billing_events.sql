-- name: CreateBillingEvent :exec
-- Append a row to the revenue ledger. The table is append-only; there
-- is no upsert/update path.
INSERT INTO billing_events (
    id, account_id, event_type, plan, amount_cents, currency, occurred_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
);

-- name: AggregateBillingMetrics :one
-- One round-trip that returns the raw numbers MRR/churn/LTV are
-- derived from:
--   mrr_brl_cents   — sum of the latest non-churn BRL amount per account
--   mrr_usd_cents   — same for USD accounts (US-018 review fixes: BRL and
--                     USD cents must never be added together raw — the
--                     use case converts USD→BRL at the owner-managed rate)
--   active_accounts — accounts whose most-recent event isn't a churn
--   churned_window  — churn events since the lookback cutoff
-- The `latest` CTE picks the most recent event per account via
-- DISTINCT ON; an account whose latest event is `churned` contributes
-- to neither MRR nor active_accounts. Empty currency = legacy pre-USD
-- rows, which were all BRL. Any other currency is excluded from both
-- sums (never silently mislabeled) and surfaced via other_currency_accounts.
WITH latest AS (
    SELECT DISTINCT ON (account_id)
        account_id, event_type, amount_cents, LOWER(currency) AS currency
    FROM billing_events
    ORDER BY account_id, occurred_at DESC, created_at DESC
)
SELECT
    COALESCE(SUM(amount_cents) FILTER (WHERE event_type <> 'churned' AND currency IN ('', 'brl')), 0)::bigint AS mrr_brl_cents,
    COALESCE(SUM(amount_cents) FILTER (WHERE event_type <> 'churned' AND currency = 'usd'), 0)::bigint AS mrr_usd_cents,
    COUNT(*) FILTER (WHERE event_type <> 'churned')::int AS active_accounts,
    COUNT(*) FILTER (WHERE event_type <> 'churned' AND currency NOT IN ('', 'brl', 'usd'))::int AS other_currency_accounts,
    (
        SELECT COUNT(*)::int
        FROM billing_events
        WHERE event_type = 'churned'
          AND occurred_at >= sqlc.arg(since)::timestamptz
    ) AS churned_window
FROM latest;
