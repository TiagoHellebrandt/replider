-- Revert to original plan prices and names.
UPDATE plans SET name = 'Starter', price = 29700, max_conversations_per_month = 1000,  max_ai_tokens_per_month = 100000,  max_agents = 1,  max_channels = 2  WHERE id = '01PLAN_BRONZE_000000000';
UPDATE plans SET name = 'Pro',     price = 69700, max_conversations_per_month = 3000,  max_ai_tokens_per_month = 250000,  max_agents = 3,  max_channels = 5  WHERE id = '01PLAN_SILVER_000000000';
UPDATE plans SET name = 'Business',price = 149700,max_conversations_per_month = 8000,  max_ai_tokens_per_month = 600000,  max_agents = 5,  max_channels = 10 WHERE id = '01PLAN_GOLD_0000000000';
UPDATE plans SET name = 'Diamond', price = 249700,max_conversations_per_month = 20000, max_ai_tokens_per_month = 1500000, max_agents = 10, max_channels = 20 WHERE id = '01PLAN_DIAMOND_00000000';
