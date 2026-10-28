ALTER TABLE plans
    DROP COLUMN IF EXISTS stripe_service_item_id,
    DROP COLUMN IF EXISTS stripe_authentication_item_id,
    DROP COLUMN IF EXISTS stripe_utility_item_id,
    DROP COLUMN IF EXISTS stripe_marketing_item_id;
DROP INDEX IF EXISTS idx_wa_billing_meter_pending;
DROP INDEX IF EXISTS uniq_wa_billing_meter;
DROP TABLE IF EXISTS wa_billing_meter;
