-- BYOK + Enterprise plan groundwork.
--
-- 1. Per-account override columns. Contract plans (Partner / Enterprise)
--    are sold with custom limits hammered out per deal — the override
--    fields let admins record those numbers without minting a brand new
--    `plans` row for every customer. NULL means "use the plan default".
-- 2. Encrypted BYOK API keys. Both columns store AES-256-GCM ciphertext
--    (nonce|ciphertext|tag, base64). Plain TEXT here would be readable
--    by anyone with a DB shell — and these are customer-funded LLM
--    spend keys; treat them like passwords.
-- 3. Enterprise plan row. `stripe_price_id` is NULL on purpose so the
--    self-serve checkout flow short-circuits to "Falar com especialista"
--    and the AI factory falls back to the account's BYOK key.
ALTER TABLE accounts
  ADD COLUMN openai_api_key_encrypted TEXT,
  ADD COLUMN gemini_api_key_encrypted TEXT,
  ADD COLUMN max_agents_override                  INT,
  ADD COLUMN max_channels_override                INT,
  ADD COLUMN max_seats_override                   INT,
  ADD COLUMN max_conversations_per_month_override INT,
  ADD COLUMN max_storage_mb_override              INT;

INSERT INTO plans (
  id, name, stripe_price_id, currency, price,
  ai_provider, ai_model, input_multiplier, output_multiplier,
  max_agents, max_channels, max_seats,
  max_conversations_per_month, max_ai_tokens_per_month, max_storage_mb,
  max_broadcast_messages_per_month, max_recipients_per_campaign,
  overage_conversation_cents, overage_cap_multiplier
) VALUES (
  '01PLAN_ENTERPRISE_00000', 'Enterprise', NULL, 'brl', 0,
  'google', 'gemini-2.5-flash', 1.0, 1.0,
  -- Limit columns are 0 → the BYOK accessor on `accounts` interprets a
  -- contract plan with 0 as "unlimited" unless an override was set.
  0, 0, 0,
  0, 0, 0,
  0, 0,
  0, 0
)
ON CONFLICT (id) DO NOTHING;
