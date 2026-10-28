-- Seed plans table with pricing tiers.
-- 1 credit = 1 token (transparent billing).
-- stripe_price_id must be set after creating Stripe Price objects.

INSERT INTO plans (id, name, stripe_price_id, currency, price, ai_provider, ai_model, input_multiplier, output_multiplier, max_agents, max_channels, max_conversations_per_month, max_ai_tokens_per_month, max_storage_mb)
VALUES
  ('01PLAN_BRONZE_000000000', 'Bronze',  NULL, 'brl',  29700, 'google', 'gemini-3.1-flash-live-preview', 1.0, 1.0,  1,   2,   1000,   100000,    10240),
  ('01PLAN_SILVER_000000000', 'Silver',  NULL, 'brl',  69700, 'google', 'gemini-3.1-flash-live-preview', 1.0, 1.0,  3,   5,   3000,   250000,    51200),
  ('01PLAN_GOLD_0000000000',  'Gold',    NULL, 'brl', 149700, 'google', 'gemini-3.1-flash-live-preview', 1.0, 1.0,  5,  10,   8000,   600000,   204800),
  ('01PLAN_DIAMOND_00000000', 'Diamond', NULL, 'brl', 249700, 'openai', 'gpt-5.4-mini',                 1.0, 1.0, 10,  20,  20000,  1500000,  1048576)
ON CONFLICT (id) DO NOTHING;
