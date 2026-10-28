-- Revert to the 4-tier grid from migration 000013 (R$ 197/497/997/1997)
-- plus the original model/seats/quotas from 000002 + 000029.
-- The two new rows (Solo, Partner) are dropped outright.

DELETE FROM plans WHERE id IN ('01PLAN_SOLO_00000000000', '01PLAN_PARTNER_00000000');

UPDATE plans SET
  name                        = 'Starter',
  price                       = 19700,
  ai_provider                 = 'google',
  ai_model                    = 'gemini-3.1-flash-live-preview',
  max_agents                  = 1,
  max_channels                = 2,
  max_seats                   = 1,
  max_conversations_per_month = 1000,
  max_ai_tokens_per_month     = 100000,
  max_storage_mb              = 10240,
  updated_at                  = NOW()
WHERE id = '01PLAN_BRONZE_000000000';

UPDATE plans SET
  name                        = 'Pro',
  price                       = 49700,
  ai_provider                 = 'google',
  ai_model                    = 'gemini-3.1-flash-live-preview',
  max_agents                  = 3,
  max_channels                = 5,
  max_seats                   = 3,
  max_conversations_per_month = 3000,
  max_ai_tokens_per_month     = 250000,
  max_storage_mb              = 51200,
  updated_at                  = NOW()
WHERE id = '01PLAN_SILVER_000000000';

UPDATE plans SET
  name                        = 'Business',
  price                       = 99700,
  ai_provider                 = 'google',
  ai_model                    = 'gemini-3.1-flash-live-preview',
  max_agents                  = 5,
  max_channels                = 10,
  max_seats                   = 5,
  max_conversations_per_month = 8000,
  max_ai_tokens_per_month     = 600000,
  max_storage_mb              = 204800,
  updated_at                  = NOW()
WHERE id = '01PLAN_GOLD_0000000000';

UPDATE plans SET
  name                        = 'Diamond',
  price                       = 199700,
  ai_provider                 = 'openai',
  ai_model                    = 'gpt-5.4-mini',
  max_agents                  = 10,
  max_channels                = 20,
  max_seats                   = 10,
  max_conversations_per_month = 20000,
  max_ai_tokens_per_month     = 1500000,
  max_storage_mb              = 1048576,
  updated_at                  = NOW()
WHERE id = '01PLAN_DIAMOND_00000000';
