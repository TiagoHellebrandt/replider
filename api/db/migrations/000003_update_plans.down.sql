-- Revert plan names: Starter -> Bronze, Pro -> Silver, Business -> Gold
UPDATE plans SET name = 'Bronze' WHERE id = '01PLAN_BRONZE_000000000';
UPDATE plans SET name = 'Silver' WHERE id = '01PLAN_SILVER_000000000';
UPDATE plans SET name = 'Gold' WHERE id = '01PLAN_GOLD_0000000000';
