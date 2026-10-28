DROP INDEX IF EXISTS idx_users_seller_ref_code;
ALTER TABLE users DROP COLUMN IF EXISTS seller_ref_code;
