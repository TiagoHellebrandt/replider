-- Update plan prices and limits based on revised market positioning (April 2026).
-- New pricing (BRL cents): Starter R$197, Pro R$497, Business R$997, Scale R$1.997
-- Renamed Diamond -> Scale to align with product positioning.

UPDATE plans SET name = 'Starter', price = 19700, max_conversations_per_month = 500,  max_ai_tokens_per_month = 75000,   max_agents = 1,  max_channels = 2  WHERE id = '01PLAN_BRONZE_000000000';
UPDATE plans SET name = 'Pro',     price = 49700, max_conversations_per_month = 3000,  max_ai_tokens_per_month = 250000,  max_agents = 3,  max_channels = 5  WHERE id = '01PLAN_SILVER_000000000';
UPDATE plans SET name = 'Business',price = 99700, max_conversations_per_month = 10000, max_ai_tokens_per_month = 600000,  max_agents = 7,  max_channels = 10 WHERE id = '01PLAN_GOLD_0000000000';
UPDATE plans SET name = 'Scale',   price = 199700,max_conversations_per_month = 30000, max_ai_tokens_per_month = 1500000, max_agents = 15, max_channels = 20 WHERE id = '01PLAN_DIAMOND_00000000';
