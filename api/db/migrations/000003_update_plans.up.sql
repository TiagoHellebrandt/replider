-- Update plan names: Bronze -> Starter, Silver -> Pro, Gold -> Business
UPDATE plans SET name = 'Starter' WHERE id = '01PLAN_BRONZE_000000000';
UPDATE plans SET name = 'Pro' WHERE id = '01PLAN_SILVER_000000000';
UPDATE plans SET name = 'Business' WHERE id = '01PLAN_GOLD_0000000000';
-- Diamond and Enterprise stay the same
