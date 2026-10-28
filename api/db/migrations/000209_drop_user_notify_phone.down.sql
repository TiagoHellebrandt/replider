ALTER TABLE users ADD COLUMN IF NOT EXISTS notify_phone TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_notify_phone_per_account
    ON users (account_id, notify_phone)
    WHERE notify_phone IS NOT NULL;
