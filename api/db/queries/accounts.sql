-- name: GetAccount :one
SELECT * FROM accounts
WHERE id = $1 LIMIT 1;

-- name: CreateAccount :exec
INSERT INTO accounts (
    id, name, currency, plan_id, stripe_customer_id, stripe_subscription_id, subscription_status, trial_expires_at, utm_source, utm_medium, utm_campaign, signup_ip, signup_country, signup_locale, created_at, updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
);

-- name: SetAccountCurrency :exec
-- Owner-panel escape hatch for locale-based currency arbitrage
-- (US-015 review fixes). The use case guards against accounts with an
-- active Stripe subscription before calling.
UPDATE accounts
SET currency = $2, updated_at = NOW()
WHERE id = $1;

-- name: GetAccountByStripeSubscription :one
SELECT * FROM accounts 
WHERE stripe_subscription_id = $1 LIMIT 1;

-- name: UpdateAccountSubscription :exec
UPDATE accounts
SET stripe_customer_id = COALESCE($2, stripe_customer_id),
    stripe_subscription_id = COALESCE($3, stripe_subscription_id),
    subscription_status = $4,
    -- Auto-manage past_due_at: stamp on first past_due, clear on recovery
    past_due_at = CASE
      WHEN $4 = 'past_due' AND past_due_at IS NULL THEN NOW()
      WHEN $4 != 'past_due' THEN NULL
      ELSE past_due_at
    END,
    updated_at = NOW()
WHERE id = $1;

-- name: SetStripeCustomerIfAbsent :one
-- Atomically claim a Stripe customer id for an account. COALESCE keeps any
-- id already persisted, so two concurrent checkouts (each having minted a
-- Stripe customer) converge on the first one written — the row lock
-- serializes the writes and the loser's id is discarded by the caller.
-- Returns the effective (winning) customer id.
UPDATE accounts
SET stripe_customer_id = COALESCE(stripe_customer_id, @customer_id),
    updated_at = NOW()
WHERE id = @id
RETURNING stripe_customer_id;

-- name: ClearTrialExpiresAt :exec
-- Called after a successful upgrade to a paid plan — the trial deadline is
-- no longer relevant since subscription_status now governs access.
UPDATE accounts
SET trial_expires_at = NULL, updated_at = NOW()
WHERE id = $1;

-- name: UpdateAccountName :exec
UPDATE accounts
SET name = $2, updated_at = NOW()
WHERE id = $1;

-- name: UpdateAccountBranding :exec
-- Touches logo_url (nullable) and timezone (NOT NULL with default).
-- Callers pass the resolved values — use COALESCE in Go, not SQL, so
-- null means "clear the logo" semantically.
UPDATE accounts
SET logo_url = $2, timezone = $3, updated_at = NOW()
WHERE id = $1;

-- name: CompleteOnboarding :exec
UPDATE accounts
SET onboarding_completed = true, updated_at = NOW()
WHERE id = $1;

-- name: UpdateAccountCSATEnabled :exec
-- Kill switch for the post-conversation CSAT asker. Flipping to false
-- stops the worker from creating new pending rows; existing pending
-- rows remain so in-flight surveys still record answers.
UPDATE accounts
SET csat_enabled = $2, updated_at = NOW()
WHERE id = $1;

-- name: DeleteAccount :exec
DELETE FROM accounts WHERE id = $1;

-- name: SetAccountTrialBypass :exec
-- Admin-only flip of trial_bypass + reason. Used for the owner's
-- self-test account and Meta App Reviewer accounts so they don't
-- need a Stripe subscription to drive the full happy path. Plan
-- limits still apply normally — only the trial-expiration gate is
-- short-circuited.
UPDATE accounts
SET trial_bypass = sqlc.arg(bypass)::boolean,
    trial_bypass_reason = NULLIF(sqlc.arg(reason)::text, ''),
    updated_at = NOW()
WHERE id = sqlc.arg(id)::char(26);

-- name: MarkPreviewChatTested :exec
-- Idempotent: only flips the flag the first time. Cheap to call from the
-- preview-chat handler on every turn.
UPDATE accounts
SET onboarding_preview_chat_tested = true, updated_at = NOW()
WHERE id = $1 AND onboarding_preview_chat_tested = false;

-- name: MarkFirstInboundReceived :exec
-- Idempotent: only writes the timestamp the first time. Called from the
-- inbound webhook pipeline; failure here must not block message
-- processing (the webhook handler logs and continues).
UPDATE accounts
SET onboarding_first_inbound_at = $2, updated_at = NOW()
WHERE id = $1 AND onboarding_first_inbound_at IS NULL;

-- name: DismissOnboardingChecklist :exec
UPDATE accounts
SET onboarding_checklist_dismissed_at = $2, updated_at = NOW()
WHERE id = $1;

-- name: GetOnboardingProgress :one
-- Single round-trip: derive every milestone from EXISTS so we don't duplicate
-- the source-of-truth logic, and pull the persisted flags + timestamps from the
-- accounts row.
--
-- The SALES milestones (products, customers, proposal) are what the dashboard
-- checklist counts. The agent/knowledge/preview ones stay in the payload because
-- they are still real facts and the `full` onboarding cohort renders them — they
-- are simply no longer what "getting started" means for this product.
SELECT
  a.onboarding_completed,
  a.onboarding_preview_chat_tested,
  a.onboarding_first_inbound_at,
  a.onboarding_checklist_dismissed_at,
  a.onboarding_current_step,
  a.onboarding_draft,
  EXISTS(SELECT 1 FROM agents ag WHERE ag.account_id = a.id)    AS agent_created,
  EXISTS(SELECT 1 FROM channels ch WHERE ch.account_id = a.id)  AS channel_connected,
  EXISTS(
    SELECT 1 FROM knowledge_sources ks
    JOIN agents a2 ON a2.id = ks.agent_id
    WHERE a2.account_id = a.id
  ) AS knowledge_added,
  -- What she sells. Nothing in this product works without it.
  EXISTS(SELECT 1 FROM products p WHERE p.account_id = a.id) AS products_added,
  -- Somebody to sell to, however they arrived: the Sent folder, prospecting, or
  -- a manual entry. The checklist asks whether the CRM has anyone in it, not
  -- which door they came through.
  EXISTS(SELECT 1 FROM contacts c WHERE c.account_id = a.id) AS customers_added,
  -- A priced offer composed at least once. The first three milestones are setup;
  -- this is the first time the product did the thing it exists for.
  EXISTS(SELECT 1 FROM proposals pr WHERE pr.account_id = a.id) AS proposal_made,
  -- Somebody besides the owner works here. The seller who will approve the
  -- offers is usually not the person who signed up; until she is invited the
  -- owner is doing a job that is not theirs.
  (SELECT count(*) FROM users u WHERE u.account_id = a.id) > 1 AS team_invited
FROM accounts a
WHERE a.id = $1
LIMIT 1;

-- name: UpdateOnboardingDraft :exec
-- Used by the Flutter client to save progress as the user moves between
-- steps or edits draft fields. Step and draft are updated atomically so
-- a partial write can't leave the checklist pointing at the wrong page.
UPDATE accounts
SET onboarding_current_step = $2,
    onboarding_draft = $3,
    updated_at = NOW()
WHERE id = $1;

-- name: SetAccountAPIKeys :exec
-- BYOK admin-only update for contract plans. Both columns are nullable
-- ciphertext: passing empty strings clears the slot, otherwise we store
-- AES-256-GCM-sealed bytes produced by internal/adapters/secrets. NULL
-- maps to "no key configured" — handled by the AI factory fallback.
UPDATE accounts
SET openai_api_key_encrypted = NULLIF(sqlc.arg(openai_key)::text, ''),
    gemini_api_key_encrypted = NULLIF(sqlc.arg(gemini_key)::text, ''),
    updated_at = NOW()
WHERE id = sqlc.arg(id)::char(26);

-- name: SetAccountLimitOverrides :exec
-- Admin-only override of per-account caps for contract customers. NULL
-- on any column resets that dimension to the plan default. Pass values
-- as Go *int (sqlc surfaces this via a pgtype.Int4-style argument);
-- callers want to send NULL to clear, so we use COALESCE-friendly
-- semantics by storing exactly what was passed.
UPDATE accounts
SET max_agents_override                       = sqlc.narg(max_agents_override)::int,
    max_channels_override                     = sqlc.narg(max_channels_override)::int,
    max_seats_override                        = sqlc.narg(max_seats_override)::int,
    max_conversations_per_month_override      = sqlc.narg(max_conversations_per_month_override)::int,
    max_storage_mb_override                   = sqlc.narg(max_storage_mb_override)::int,
    max_ai_tokens_per_month_override          = sqlc.narg(max_ai_tokens_per_month_override)::int,
    max_broadcast_messages_per_month_override = sqlc.narg(max_broadcast_messages_per_month_override)::int,
    max_recipients_per_campaign_override      = sqlc.narg(max_recipients_per_campaign_override)::int,
    updated_at                                = NOW()
WHERE id = sqlc.arg(id)::char(26);

-- name: SuspendAccount :exec
-- Owner-only freeze. Future inbound webhooks short-circuit with a
-- 403 ACCOUNT_SUSPENDED until ReactivateAccount clears the row.
UPDATE accounts
SET suspended_at = NOW(),
    suspended_reason = sqlc.arg(reason)::text,
    updated_at = NOW()
WHERE id = sqlc.arg(id)::char(26);

-- name: ReactivateAccount :exec
UPDATE accounts
SET suspended_at = NULL,
    suspended_reason = NULL,
    updated_at = NOW()
WHERE id = sqlc.arg(id)::char(26);

-- name: ListAccountsForOwner :many
-- Owner-panel listing. Filters: search (name OR user email), status
-- ('' / 'active' / 'suspended' / 'trialing' / 'past_due'). Returns
-- the bare account row + plan_name + plan token cap + admin
-- email/name + current-period token usage for the cap-alert badge.
SELECT
    a.*,
    p.name AS plan_name,
    COALESCE(p.max_ai_tokens_per_month, 0) AS plan_max_ai_tokens_per_month,
    COALESCE((SELECT u.email FROM users u
        WHERE u.account_id = a.id AND u.role IN ('admin', 'owner')
        ORDER BY u.created_at ASC LIMIT 1), '')::text AS owner_email,
    COALESCE((SELECT u.name FROM users u
        WHERE u.account_id = a.id AND u.role IN ('admin', 'owner')
        ORDER BY u.created_at ASC LIMIT 1), '')::text AS owner_name,
    COALESCE(au.ai_tokens_used, 0) AS tokens_used,
    COALESCE(au.addon_tokens, 0) AS addon_tokens,
    COALESCE(au.conversations_count, 0) AS conversations_count,
    -- Worst WhatsApp quality rating across the account's numbers, so the
    -- owner list can flag a number at risk of being banned (RED) without
    -- opening each account. Cached on channels.credentials by the
    -- WhatsAppQualityWorker; NULL until the first probe lands.
    COALESCE((SELECT c.credentials->>'quality_rating'
        FROM channels c
        WHERE c.account_id = a.id AND c.provider = 'whatsapp'
          AND c.credentials->>'quality_rating' IS NOT NULL
        ORDER BY CASE c.credentials->>'quality_rating'
                   WHEN 'RED' THEN 0 WHEN 'YELLOW' THEN 1
                   WHEN 'GREEN' THEN 2 ELSE 3 END
        LIMIT 1), '')::text AS whatsapp_quality
FROM accounts a
LEFT JOIN plans p ON p.id = a.plan_id
LEFT JOIN account_usages au ON au.account_id = a.id
WHERE
    (
        sqlc.arg(search)::text = ''
        OR a.name ILIKE '%' || sqlc.arg(search)::text || '%'
        OR EXISTS (
            SELECT 1 FROM users u
            WHERE u.account_id = a.id
              AND u.email ILIKE '%' || sqlc.arg(search)::text || '%'
        )
    )
    AND (
        sqlc.arg(status)::text = ''
        OR (sqlc.arg(status)::text = 'suspended' AND a.suspended_at IS NOT NULL)
        OR (sqlc.arg(status)::text = 'active' AND a.suspended_at IS NULL AND a.subscription_status = 'active')
        OR (sqlc.arg(status)::text = 'trialing' AND a.suspended_at IS NULL AND a.subscription_status = 'trialing')
        OR (sqlc.arg(status)::text = 'past_due' AND a.suspended_at IS NULL AND a.subscription_status = 'past_due')
    )
ORDER BY a.created_at DESC
LIMIT sqlc.arg(lim)::int OFFSET sqlc.arg(off)::int;

-- name: CountAccountsForOwner :one
SELECT COUNT(*) FROM accounts a
WHERE
    (
        sqlc.arg(search)::text = ''
        OR a.name ILIKE '%' || sqlc.arg(search)::text || '%'
        OR EXISTS (
            SELECT 1 FROM users u
            WHERE u.account_id = a.id
              AND u.email ILIKE '%' || sqlc.arg(search)::text || '%'
        )
    )
    AND (
        sqlc.arg(status)::text = ''
        OR (sqlc.arg(status)::text = 'suspended' AND a.suspended_at IS NOT NULL)
        OR (sqlc.arg(status)::text = 'active' AND a.suspended_at IS NULL AND a.subscription_status = 'active')
        OR (sqlc.arg(status)::text = 'trialing' AND a.suspended_at IS NULL AND a.subscription_status = 'trialing')
        OR (sqlc.arg(status)::text = 'past_due' AND a.suspended_at IS NULL AND a.subscription_status = 'past_due')
    );

-- name: SetAccountSubscriptionStatus :exec
-- Force-set subscription_status independent of Stripe — used by the
-- owner panel to override e.g. mark a contract account 'active' when
-- it isn't backed by a Stripe subscription.
UPDATE accounts
SET subscription_status = sqlc.arg(status)::text,
    updated_at = NOW()
WHERE id = sqlc.arg(id)::char(26);

-- name: SetAccountPlan :exec
-- Owner panel: assign a plan to an existing account (apply a contract
-- to an already-registered account). Independent of Stripe — the
-- contract flow pairs this with SetAccountSubscriptionStatus('active').
UPDATE accounts
SET plan_id = sqlc.arg(plan_id)::char(26),
    updated_at = NOW()
WHERE id = sqlc.arg(id)::char(26);

-- name: SetAccountPlatformKeyAllowed :exec
-- Owner panel: allow/forbid a contract account to run on the platform AI
-- key instead of BYOK (Enterprise "our key" deals).
UPDATE accounts
SET platform_key_allowed = sqlc.arg(platform_key_allowed)::boolean,
    updated_at = NOW()
WHERE id = sqlc.arg(id)::char(26);

-- name: TopAccountsByTokens :many
-- Owner-panel "top consumers" listing. Joins the bare account row with
-- the active-period usage so the operator can spot heavy users at a
-- glance. Defaults to current-period tokens; result is ordered DESC
-- so the heaviest is row 0.
SELECT
    a.id,
    a.name,
    a.plan_id,
    p.name AS plan_name,
    COALESCE(au.ai_tokens_used, 0) AS tokens_used,
    COALESCE(au.conversations_count, 0) AS conversations_count,
    a.suspended_at
FROM accounts a
LEFT JOIN plans p ON p.id = a.plan_id
LEFT JOIN account_usages au ON au.account_id = a.id
ORDER BY COALESCE(au.ai_tokens_used, 0) DESC, a.created_at DESC
LIMIT sqlc.arg(lim)::int;

-- name: ExtendTrial :exec
-- Owner-only push of accounts.trial_expires_at into the future. Pass an
-- already-resolved timestamptz from the use case (NOW() + N days) so the
-- query stays declarative.
UPDATE accounts
SET trial_expires_at = sqlc.arg(new_expires_at)::timestamptz,
    updated_at = NOW()
WHERE id = sqlc.arg(id)::char(26);

-- name: SystemHealthSummary :one
-- Owner-panel system health snapshot. Aggregate counts only — no
-- per-row data, no joins. Cheap enough to call on every panel open.
SELECT
    (SELECT COUNT(*) FROM accounts)::bigint AS accounts_total,
    (SELECT COUNT(*) FROM accounts WHERE suspended_at IS NULL AND subscription_status = 'active')::bigint AS accounts_active,
    (SELECT COUNT(*) FROM accounts WHERE subscription_status = 'trialing')::bigint AS accounts_trialing,
    (SELECT COUNT(*) FROM accounts WHERE suspended_at IS NOT NULL)::bigint AS accounts_suspended,
    (SELECT COUNT(*) FROM users WHERE is_active = TRUE)::bigint AS users_active,
    (SELECT COUNT(*) FROM conversations WHERE created_at > NOW() - INTERVAL '24 hours')::bigint AS conversations_24h,
    (SELECT COUNT(*) FROM messages WHERE created_at > NOW() - INTERVAL '24 hours')::bigint AS messages_24h;

-- name: CountActiveInternalThreads :one
-- Owner-panel account detail (canal-interno US-010): how many OPEN
-- internal CS threads this customer account's users currently hold.
-- customer_account_id is stamped exclusively on internal-channel
-- conversations (US-002), so no channel join is needed; the partial
-- index from migration 000142 covers the filter.
SELECT COUNT(*)::bigint AS active_threads
FROM conversations
WHERE customer_account_id = sqlc.arg(customer_account_id)::char(26)
  AND status = 'open';

-- name: UsageTrendByAccount :many
-- 30-day token + conversation count rollup for the owner-panel chart.
-- tokens_in/tokens_out come from the messages.metadata JSONB stamped
-- by the inbound pipeline; days with no AI messages return 0 (the
-- generate_series ensures every day in the window has a row).
WITH days AS (
    SELECT generate_series(
        date_trunc('day', NOW() - INTERVAL '29 days'),
        date_trunc('day', NOW()),
        INTERVAL '1 day'
    )::date AS day
)
SELECT
    days.day::date AS day,
    COALESCE(SUM(
        COALESCE((m.metadata->>'tokens_in')::bigint, 0) +
        COALESCE((m.metadata->>'tokens_out')::bigint, 0)
    ), 0)::bigint AS tokens,
    COUNT(DISTINCT m.conversation_id)::bigint AS conversations
FROM days
LEFT JOIN messages m ON date_trunc('day', m.created_at)::date = days.day
    AND m.conversation_id IN (
        SELECT id FROM conversations WHERE account_id = sqlc.arg(account_id)::char(26)
    )
GROUP BY days.day
ORDER BY days.day ASC;

-- name: UpdateSubscriptionPeriodEnd :exec
-- Cache the Stripe subscription's current_period_end so the pre-billing
-- cron (US-124) can find accounts whose next charge is approaching.
-- Synced from customer.subscription.{created,updated} webhooks.
UPDATE accounts
SET current_period_end = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: ListAccountsDueForPrebilling :many
-- Active accounts whose next charge is within the notification window
-- (current_period_end between NOW and the supplied cutoff) and which
-- haven't already been emailed for THIS billing cycle. The
-- prebilling_notified_period_end <> current_period_end guard makes the
-- cron idempotent across repeated ticks inside the 72h window.
SELECT id, plan_id, current_period_end
FROM accounts
WHERE subscription_status = 'active'
  AND current_period_end IS NOT NULL
  AND current_period_end > NOW()
  AND current_period_end <= sqlc.arg(cutoff)::timestamptz
  AND (prebilling_notified_period_end IS NULL
       OR prebilling_notified_period_end <> current_period_end)
ORDER BY current_period_end ASC
LIMIT 500;

-- name: MarkPrebillingNotified :exec
-- Stamp the period_end we just emailed about so the cron won't resend
-- for the same cycle. Renewal moves current_period_end forward, which
-- re-arms the reminder for the next cycle.
UPDATE accounts
SET prebilling_notified_period_end = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: ListAccountIDsWithActiveSubscription :many
-- Self-serve accounts on an active Stripe subscription, used by the
-- overage-reporting worker (US-168) to push metered usage. Filters to
-- rows that actually have a Stripe subscription + a plan so the reporter
-- never wastes a Stripe round-trip on trial/contract accounts.
SELECT id
FROM accounts
WHERE subscription_status = 'active'
  AND stripe_subscription_id IS NOT NULL
  AND stripe_subscription_id <> ''
  AND plan_id IS NOT NULL
ORDER BY id ASC;

-- name: SetReplyReminderSettings :exec
-- Flips per-account opt-in + threshold for the reply-reminder worker.
-- The CHECK constraint on the column rejects out-of-range minutes so
-- callers see a SQL-level error instead of corrupt state.
UPDATE accounts
SET reply_reminder_enabled = $2,
    reply_reminder_threshold_minutes = $3,
    updated_at = NOW()
WHERE id = $1;

-- name: SetAccountSeller :exec
-- Links (or re-links) the account to a seller from the platform
-- account. seller_linked_at is stamped on every (re)link so carteira
-- history reads correctly after a manual reallocation.
UPDATE accounts
SET seller_user_id = $2,
    lead_source = $3,
    seller_linked_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: ClearAccountSeller :exec
-- Unlinks the seller entirely (manual owner action). All three columns
-- go NULL together — a lead_source without a seller is meaningless.
UPDATE accounts
SET seller_user_id = NULL,
    lead_source = NULL,
    seller_linked_at = NULL,
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateAccountBusinessProfile :exec
-- What the company does and where. Callers pass the resolved values: an
-- empty array clears the lines, a NULL territory clears the territory.
UPDATE accounts
SET business_lines = $2, territory = $3, updated_at = NOW()
WHERE id = $1;

-- name: SetAccountAIMonthlyBudget :exec
UPDATE accounts SET ai_monthly_budget_cents = $2, updated_at = NOW() WHERE id = $1;

-- name: ListSellingTerritories :many
-- Every distinct territory somebody actually sells into.
--
-- The contract sweep is shared work: a hospital's public contracts are the
-- same public record for every supplier in that state, so two accounts in Mato
-- Grosso cost one sweep, not two. Suspended accounts are excluded — sweeping
-- for somebody who cannot log in spends a free government API's patience on
-- nobody's behalf.
SELECT DISTINCT territory
FROM accounts
WHERE territory IS NOT NULL
  AND suspended_at IS NULL;
