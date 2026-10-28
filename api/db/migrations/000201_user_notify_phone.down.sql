DROP INDEX IF EXISTS idx_users_notify_phone_per_account;
ALTER TABLE users DROP COLUMN IF EXISTS notify_phone;
