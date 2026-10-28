-- SDR tiers (US-M013).
--
-- The old ladder priced ATTENDANCE (conversations, agents). The product now
-- sells PROSPECTING, so the caps that matter are different: how many new
-- leads a month, how much enrichment, how many warmed mailboxes, how many
-- funnels. Those get real columns because they are enforced; "cadências
-- ativas" from the pricing table deliberately does NOT, because the feature
-- does not exist yet and a column nothing reads is a promise nothing keeps.
--
-- 0 keeps its existing meaning across the plan columns: "not allowed" for a
-- self-serve plan, "use the account override" for a contract plan
-- (resolveLimit in domain/account.go).
ALTER TABLE plans
    ADD COLUMN IF NOT EXISTS max_leads_per_month INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_enrichments_per_month INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_email_mailboxes INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_funnels INTEGER NOT NULL DEFAULT 0;

-- The three SDR tiers. Prices in cents, BRL.
--
-- max_channels/max_agents are generous rather than the old ladder's tight
-- caps: the SDR product's scarcity is leads and mailboxes, not agents, and
-- re-using the old caps would make a R$797 plan feel smaller than the R$399
-- one it replaces.
--
-- Token allowances are sized off the pricing doc's COGS assumptions with
-- headroom, and max_daily_ai_tokens is monthly/5 (the same rule migration
-- 000139 used) so the daily anti-runaway cap can never throttle an account
-- below its marketed allowance.
INSERT INTO plans (
  id, name, stripe_price_id, stripe_price_id_annual, currency, price, is_listed,
  ai_provider, ai_model, input_multiplier, output_multiplier,
  max_agents, max_channels, max_seats,
  max_conversations_per_month, max_ai_tokens_per_month, max_daily_ai_tokens, max_storage_mb,
  max_broadcast_messages_per_month, max_recipients_per_campaign,
  overage_conversation_cents, overage_cap_multiplier,
  max_leads_per_month, max_enrichments_per_month, max_email_mailboxes, max_funnels
) VALUES
  (
    '01PLAN_SDR_VENDEDOR_000', 'Vendedor', NULL, NULL, 'brl', 79700, TRUE,
    'google', 'gemini-3.1-flash-lite', 1.0, 1.0,
    2, 3, 1,
    1000, 30000000, 6000000, 10240,
    1000, 500,
    0, 0,
    200, 200, 2, 1
  ),
  (
    '01PLAN_SDR_EQUIPE_00000', 'Equipe', NULL, NULL, 'brl', 149700, TRUE,
    'google', 'gemini-3.1-flash-lite', 1.0, 1.0,
    5, 999, 3,
    3000, 75000000, 15000000, 30720,
    3000, 1500,
    0, 0,
    500, 500, 4, 999
  ),
  (
    '01PLAN_SDR_ESCALA_00000', 'Escala', NULL, NULL, 'brl', 299700, TRUE,
    'google', 'gemini-3.1-flash-lite', 1.0, 1.0,
    15, 999, 10,
    10000, 200000000, 40000000, 102400,
    10000, 5000,
    0, 0,
    1500, 1500, 12, 999
  )
ON CONFLICT (id) DO NOTHING;

-- Retire the attendance ladder from the public catalogue. Unlisted, NOT
-- deleted: GetPlanByID stays unfiltered, so every account already on one of
-- these keeps working exactly as before — this only stops new signups from
-- choosing a plan the product no longer sells.
UPDATE plans
SET is_listed = FALSE, updated_at = NOW()
WHERE id IN (
  '01PLAN_SOLO_00000000000',
  '01PLAN_BRONZE_000000000',
  '01PLAN_SILVER_000000000',
  '01PLAN_GOLD_0000000000',
  '01PLAN_DIAMOND_00000000'
);
