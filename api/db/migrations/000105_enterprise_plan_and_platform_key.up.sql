-- Enterprise contract tier + per-account "run on our key" flag.
--
-- Enterprise is a custom, per-contract tier: no Stripe price (contract =
-- BYOK-capable), higher AI cost multipliers than self-serve as the "our
-- key costs more" baseline. Each enterprise account is then tailored via
-- the per-account limit overrides set in the owner panel. Mirrors the
-- Partner row from migration 000033.
INSERT INTO plans (
  id, name, stripe_price_id, currency, price,
  ai_provider, ai_model, input_multiplier, output_multiplier,
  max_agents, max_channels, max_seats,
  max_conversations_per_month, max_ai_tokens_per_month, max_storage_mb
) VALUES (
  '01PLAN_ENTERPRISE_00000', 'Enterprise', NULL, 'brl', 0,
  'google', 'gemini-2.5-flash', 1.5, 1.5,
  100, 200, 200,
  200000, 50000000, 4194304
)
ON CONFLICT (id) DO NOTHING;

-- When true, a contract account may run on the PLATFORM AI key instead of
-- its own (BYOK). Lets an Enterprise deal run on our key (billed at the
-- plan's higher multipliers) without forcing the customer to bring keys.
-- Default false preserves today's strict-BYOK behaviour for every other
-- contract account (e.g. Partner) — AIFactory still returns
-- ErrBYOKKeyMissing for those when no key is set.
ALTER TABLE accounts
  ADD COLUMN platform_key_allowed BOOLEAN NOT NULL DEFAULT false;
