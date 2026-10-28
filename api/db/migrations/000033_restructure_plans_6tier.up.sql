-- 6-tier plan restructure (2026-04). Aligns the DB with the grid
-- validated in docs/calc_margem.html and the landing page.
-- Partner stays in the DB but is never exposed publicly (no Stripe
-- price id here — left NULL on purpose; commercial handshake only).
--
-- Prices are centavos BRL. AI model converged on gemini-2.5-flash
-- (see api/internal/core/services/ai_factory.go comments); the
-- previous seed still carried gemini-3.1-flash-live-preview and
-- this migration also aligns that column so prod + dev + code
-- agree on one model.
--
-- Names:
--   BRONZE  → Starter     (was "Starter" since 000003; keeping the DB id)
--   SILVER  → Pro
--   GOLD    → Business
--   DIAMOND → Scale
--   new     : Solo, Partner

-- Rename + reprice the four legacy tiers.
UPDATE plans SET
  name                        = 'Starter',
  price                       = 39900,
  ai_provider                 = 'google',
  ai_model                    = 'gemini-2.5-flash',
  max_agents                  = 2,
  max_channels                = 3,
  max_seats                   = 2,
  max_conversations_per_month = 700,
  max_ai_tokens_per_month     = 280000,
  max_storage_mb              = 20480,
  updated_at                  = NOW()
WHERE id = '01PLAN_BRONZE_000000000';

UPDATE plans SET
  name                        = 'Pro',
  price                       = 89900,
  ai_provider                 = 'google',
  ai_model                    = 'gemini-2.5-flash',
  max_agents                  = 5,
  max_channels                = 8,
  max_seats                   = 5,
  max_conversations_per_month = 2000,
  max_ai_tokens_per_month     = 800000,
  max_storage_mb              = 102400,
  updated_at                  = NOW()
WHERE id = '01PLAN_SILVER_000000000';

UPDATE plans SET
  name                        = 'Business',
  price                       = 199000,
  ai_provider                 = 'google',
  ai_model                    = 'gemini-2.5-flash',
  max_agents                  = 15,
  max_channels                = 20,
  max_seats                   = 15,
  max_conversations_per_month = 5000,
  max_ai_tokens_per_month     = 2000000,
  max_storage_mb              = 512000,
  updated_at                  = NOW()
WHERE id = '01PLAN_GOLD_0000000000';

UPDATE plans SET
  name                        = 'Scale',
  price                       = 399000,
  ai_provider                 = 'google',
  ai_model                    = 'gemini-2.5-flash',
  max_agents                  = 40,
  max_channels                = 50,
  max_seats                   = 40,
  max_conversations_per_month = 10000,
  max_ai_tokens_per_month     = 4000000,
  max_storage_mb              = 1048576,
  updated_at                  = NOW()
WHERE id = '01PLAN_DIAMOND_00000000';

-- Two new tiers. Solo is the new floor; Partner is commercial-only
-- (stripe_price_id stays NULL; listed in /v1/plans responses but
-- the landing + app hide it).
INSERT INTO plans (
  id, name, stripe_price_id, currency, price,
  ai_provider, ai_model, input_multiplier, output_multiplier,
  max_agents, max_channels, max_seats,
  max_conversations_per_month, max_ai_tokens_per_month, max_storage_mb
) VALUES
  (
    '01PLAN_SOLO_00000000000', 'Solo', NULL, 'brl', 19900,
    'google', 'gemini-2.5-flash', 1.0, 1.0,
    1, 2, 1,
    200, 100000, 5120
  ),
  (
    '01PLAN_PARTNER_00000000', 'Partner', NULL, 'brl', 549000,
    'google', 'gemini-2.5-flash', 1.0, 1.0,
    50, 100, 100,
    60000, 20000000, 2097152
  )
ON CONFLICT (id) DO NOTHING;
