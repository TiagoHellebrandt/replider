-- push_subscriptions holds per-device push endpoints for dashboard users.
-- Web uses endpoint + p256dh_key + auth_key (W3C Push API). Mobile (future
-- FCM/APNs) uses device_token. Keeping one table with platform discriminator
-- avoids fanning out repos when the Flutter mobile build ships.
CREATE TABLE IF NOT EXISTS push_subscriptions (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id      TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    platform        TEXT NOT NULL CHECK (platform IN ('web', 'ios', 'android')),
    endpoint        TEXT NOT NULL,
    p256dh_key      TEXT,
    auth_key        TEXT,
    device_token    TEXT,
    user_agent      TEXT,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    last_used_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT push_subscriptions_user_endpoint_unique UNIQUE (user_id, endpoint)
);

CREATE INDEX IF NOT EXISTS push_subscriptions_account_enabled_idx
    ON push_subscriptions (account_id) WHERE enabled;

CREATE INDEX IF NOT EXISTS push_subscriptions_user_idx
    ON push_subscriptions (user_id);
