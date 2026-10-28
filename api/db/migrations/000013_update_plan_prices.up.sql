-- Align plan prices with the landing page (web/src/i18n/ui.ts).
-- Prices in centavos BRL.
UPDATE plans SET price = 19700,  updated_at = NOW() WHERE id = '01PLAN_BRONZE_000000000';  -- Starter: R$ 197
UPDATE plans SET price = 49700,  updated_at = NOW() WHERE id = '01PLAN_SILVER_000000000';  -- Pro:     R$ 497
UPDATE plans SET price = 99700,  updated_at = NOW() WHERE id = '01PLAN_GOLD_0000000000';   -- Business: R$ 997
UPDATE plans SET price = 199700, updated_at = NOW() WHERE id = '01PLAN_DIAMOND_00000000';  -- Diamond: R$ 1.997
