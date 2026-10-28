-- USD self-serve tiers for the US launch (US-002). Monthly-only —
-- annual billing was deprioritized in the Vendedores pivot, so
-- stripe_price_id_annual stays NULL on purpose.
--
-- Prices are cents USD: Solo $39 / Core $99 / Pro $199. Anchors and
-- unit economics live in tasks/prd-us-product-gaps.md ("USD pricing
-- tiers" + "Unit economics"). Token allowances are 35K tokens per AI
-- conversation × the conversation allowance (Solo 200 → 7M, Core
-- 1000 → 35M, Pro 3000 → 105M) so the cap matches the marketed
-- conversation counts.
--
-- ai_model is gemini-3.1-flash-lite ($0.25/$1.50 per 1M tokens) — NOT
-- gemini-3.1-flash-live-preview, which is a Live API (audio) model
-- priced 3× higher for text.
--
-- stripe_price_id is seeded NULL on purpose. To go live:
--   1. Stripe dashboard → Products: create one Product per tier with a
--      recurring monthly Price in USD (3900/9900/19900 cents).
--   2. Wire each Price id:
--        UPDATE plans SET stripe_price_id = 'price_...', updated_at = NOW()
--        WHERE id = '01PLAN_SOLO_USD_0000000';  -- and core/pro rows
--   (cmd/seed_stripe_prices hardcodes currency 'brl' — do NOT run it
--   for these rows until it learns the plan's currency.)
-- Until the ids are filled, USD checkout fails with a clear
-- PLAN_NOT_PURCHASABLE error (no panic) and the dashboard renders the
-- contact-sales CTA. Note: a NULL stripe_price_id also makes
-- IsContractPlan treat the plan with contract semantics — harmless
-- here because no account can subscribe to an unpurchasable plan.
--
-- max_channels 999 on Pro = "unlimited" in practice. It cannot be 0:
-- resolveLimit (domain/account.go) reads a 0 plan cap as "feature
-- disabled" for self-serve (non-contract) plans and would block every
-- channel create once the Stripe price id is wired.
--
-- Broadcast caps mirror the BRL ladder (Starter 1000/500, Pro
-- 10000/5000); Solo gets 0 = broadcasts disabled, same as BRL Solo.
-- Overage metering is 0 (disabled): USD launch is flat monthly-only,
-- hard stop at the plan limit.
INSERT INTO plans (
  id, name, stripe_price_id, stripe_price_id_annual, currency, price,
  ai_provider, ai_model, input_multiplier, output_multiplier,
  max_agents, max_channels, max_seats,
  max_conversations_per_month, max_ai_tokens_per_month, max_storage_mb,
  max_broadcast_messages_per_month, max_recipients_per_campaign,
  overage_conversation_cents, overage_cap_multiplier
) VALUES
  (
    '01PLAN_SOLO_USD_0000000', 'Solo', NULL, NULL, 'usd', 3900,
    'google', 'gemini-3.1-flash-lite', 1.0, 1.0,
    1, 1, 2,
    200, 7000000, 5120,
    0, 0,
    0, 0
  ),
  (
    '01PLAN_CORE_USD_0000000', 'Core', NULL, NULL, 'usd', 9900,
    'google', 'gemini-3.1-flash-lite', 1.0, 1.0,
    3, 3, 5,
    1000, 35000000, 20480,
    1000, 500,
    0, 0
  ),
  (
    '01PLAN_PRO_USD_00000000', 'Pro', NULL, NULL, 'usd', 19900,
    'google', 'gemini-3.1-flash-lite', 1.0, 1.0,
    10, 999, 15,
    3000, 105000000, 102400,
    10000, 5000,
    0, 0
  )
ON CONFLICT (id) DO NOTHING;
