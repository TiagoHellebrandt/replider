-- Revert to original seed prices (000002_seed_plans.up.sql).
UPDATE plans SET price = 29700,  updated_at = NOW() WHERE id = '01PLAN_BRONZE_000000000';
UPDATE plans SET price = 69700,  updated_at = NOW() WHERE id = '01PLAN_SILVER_000000000';
UPDATE plans SET price = 149700, updated_at = NOW() WHERE id = '01PLAN_GOLD_0000000000';
UPDATE plans SET price = 249700, updated_at = NOW() WHERE id = '01PLAN_DIAMOND_00000000';
