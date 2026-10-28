-- name: GetUsageByAccountID :one
SELECT * FROM account_usages
WHERE account_id = $1
ORDER BY billing_cycle_start DESC
LIMIT 1;

-- name: CreateUsage :exec
INSERT INTO account_usages (
    id, account_id, billing_cycle_start, conversations_count, ai_tokens_used, storage_used_mb, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
);

-- name: UpdateUsage :exec
UPDATE account_usages
SET conversations_count = $2,
    ai_tokens_used = $3,
    storage_used_mb = $4,
    updated_at = $5
WHERE id = $1;

-- name: AddUsage :exec
-- Increment counters on the LATEST cycle row only. Filtering by
-- account_id alone would update every historical row if more than one
-- ever existed (demo seeds, partial cycle rotations), making the
-- running total nonsense — same C1 pattern fixed for overage. The
-- subquery is cheap: one indexed lookup + sort over at most a handful
-- of rows per account. The right-hand reads are qualified with the
-- outer table name to disambiguate from the subquery's `au` alias.
-- daily_tokens resets lazily (US-025): when the stored date isn't today the
-- counter restarts at this call's token delta instead of accumulating
-- yesterday's total, so no cron is needed to roll the window.
UPDATE account_usages
SET ai_tokens_used        = COALESCE(account_usages.ai_tokens_used, 0) + $2,
    ai_input_tokens_used  = COALESCE(account_usages.ai_input_tokens_used, 0) + $3,
    ai_output_tokens_used = COALESCE(account_usages.ai_output_tokens_used, 0) + $4,
    conversations_count   = COALESCE(account_usages.conversations_count, 0) + $5,
    daily_tokens = CASE
        WHEN account_usages.daily_tokens_date = CURRENT_DATE THEN COALESCE(account_usages.daily_tokens, 0) + $2
        ELSE $2
    END,
    daily_tokens_date = CURRENT_DATE,
    updated_at = NOW()
WHERE id = (
    SELECT au.id FROM account_usages au
    WHERE au.account_id = $1
    ORDER BY au.billing_cycle_start DESC
    LIMIT 1
);

-- name: AddAddonTokens :exec
-- US-012: a single `SET addon_tokens = addon_tokens + $2` is an ATOMIC
-- read-modify-write at the DB level — Postgres takes a row lock for the
-- duration of the UPDATE, so concurrent AddAddonTokens calls serialize and
-- every increment lands (no lost update). This is strictly better than an
-- application-side BEGIN; SELECT ... FOR UPDATE; UPDATE; COMMIT round-trip,
-- which would add latency for the same guarantee. Double-credit from a
-- retried Stripe event is prevented one layer up by ClaimEvent
-- (INSERT ... ON CONFLICT DO NOTHING on the event id), so the same event
-- never reaches this UPDATE twice.
UPDATE account_usages
SET addon_tokens = COALESCE(account_usages.addon_tokens, 0) + $2,
    updated_at = NOW()
WHERE id = (
    SELECT au.id FROM account_usages au
    WHERE au.account_id = $1
    ORDER BY au.billing_cycle_start DESC
    LIMIT 1
);

-- name: AddBroadcastMessages :exec
-- Increments the broadcast-messages counter for the given account. Called
-- at campaign creation time to reserve budget against the plan cap.
UPDATE account_usages
SET broadcast_messages_used = COALESCE(account_usages.broadcast_messages_used, 0) + $2,
    updated_at = NOW()
WHERE id = (
    SELECT au.id FROM account_usages au
    WHERE au.account_id = $1
    ORDER BY au.billing_cycle_start DESC
    LIMIT 1
);

-- name: ResetUsage :exec
-- Called on invoice.paid — a new billing cycle starts, so every
-- per-cycle counter (including the overage tallies and cap marker)
-- goes back to zero.
UPDATE account_usages
SET ai_tokens_used = 0,
    conversations_count = 0,
    broadcast_messages_used = 0,
    overage_conversations_count = 0,
    overage_cents_charged = 0,
    cap_reached_at = NULL,
    warned_80_percent = false,
    warned_100_percent = false,
    updated_at = NOW()
WHERE account_id = $1;

-- name: ListOverageCandidates :many
-- Self-serve (Stripe-billed) accounts on an overage-enabled plan whose
-- conversation count has passed the effective franchise. DISTINCT ON
-- keeps the latest usage row per account. The franchise honours the
-- per-account override so it matches the usage guard. Contract plans
-- (stripe_price_id IS NULL) are billed off-platform and excluded.
-- past_due accounts are included so overage accrued during the grace
-- period is still billed. overage_cap_multiplier is cast to float8 so
-- sqlc emits a plain float64 instead of pgtype.Numeric.
SELECT DISTINCT ON (a.id)
    a.id                                  AS account_id,
    a.stripe_customer_id                  AS stripe_customer_id,
    a.currency                            AS currency,
    p.price                               AS plan_price_cents,
    COALESCE(a.max_conversations_per_month_override, p.max_conversations_per_month)::int AS max_conversations_per_month,
    p.overage_conversation_cents          AS overage_conversation_cents,
    p.overage_cap_multiplier::float8      AS overage_cap_multiplier,
    COALESCE(u.conversations_count, 0)::int AS conversations_count,
    u.overage_cents_charged               AS overage_cents_charged,
    (u.cap_reached_at IS NOT NULL)::boolean AS cap_reached
FROM accounts a
JOIN plans p ON p.id = a.plan_id
JOIN account_usages u ON u.account_id = a.id
WHERE a.subscription_status IN ('active', 'past_due')
  AND a.stripe_customer_id IS NOT NULL
  AND p.stripe_price_id IS NOT NULL
  AND p.overage_conversation_cents > 0
  AND COALESCE(u.conversations_count, 0) > COALESCE(a.max_conversations_per_month_override, p.max_conversations_per_month)
ORDER BY a.id, u.billing_cycle_start DESC;

-- name: GetOverageCandidate :one
-- Single-account variant used by the invoice.paid handler to settle
-- any outstanding overage BEFORE the cycle counters are reset. No
-- franchise / status filter — the settler treats an account still
-- under the franchise as a no-op. Returns no row (skipped) for
-- contract plans or accounts with no Stripe customer / no overage rate.
SELECT DISTINCT ON (a.id)
    a.id                                  AS account_id,
    a.stripe_customer_id                  AS stripe_customer_id,
    a.currency                            AS currency,
    p.price                               AS plan_price_cents,
    COALESCE(a.max_conversations_per_month_override, p.max_conversations_per_month)::int AS max_conversations_per_month,
    p.overage_conversation_cents          AS overage_conversation_cents,
    p.overage_cap_multiplier::float8      AS overage_cap_multiplier,
    COALESCE(u.conversations_count, 0)::int AS conversations_count,
    u.overage_cents_charged               AS overage_cents_charged,
    (u.cap_reached_at IS NOT NULL)::boolean AS cap_reached
FROM accounts a
JOIN plans p ON p.id = a.plan_id
JOIN account_usages u ON u.account_id = a.id
WHERE a.id = $1
  AND a.stripe_customer_id IS NOT NULL
  AND p.stripe_price_id IS NOT NULL
  AND p.overage_conversation_cents > 0
ORDER BY a.id, u.billing_cycle_start DESC;

-- name: RecordOverageCharge :exec
-- Advances the per-cycle overage tallies after the worker bills Stripe.
-- Targets the latest usage row only, matching how every reader resolves
-- the current cycle (GetUsageByAccountID, ListOverageCandidates).
-- cap_reached_at is stamped once (CASE keeps the first timestamp).
UPDATE account_usages
SET overage_conversations_count = $2,
    overage_cents_charged       = $3,
    cap_reached_at = CASE
        WHEN sqlc.arg(cap_reached)::boolean AND cap_reached_at IS NULL
        THEN NOW() ELSE cap_reached_at END,
    updated_at = NOW()
WHERE id = (
    SELECT au.id FROM account_usages au
    WHERE au.account_id = $1
    ORDER BY au.billing_cycle_start DESC
    LIMIT 1
);

-- name: SetUsageWarned80 :exec
UPDATE account_usages SET warned_80_percent = true
WHERE id = (SELECT au.id FROM account_usages au WHERE au.account_id = $1 ORDER BY au.billing_cycle_start DESC LIMIT 1);

-- name: SetUsageWarned100 :exec
UPDATE account_usages SET warned_100_percent = true
WHERE id = (SELECT au.id FROM account_usages au WHERE au.account_id = $1 ORDER BY au.billing_cycle_start DESC LIMIT 1);

