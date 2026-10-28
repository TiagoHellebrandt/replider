-- Per-account Google Calendar OAuth integration (self-serve "Conectar
-- Google Calendar"). One row per account; the refresh token is sealed
-- with the BYOK AES-256-GCM cipher (text form), same class as the other
-- third-party API credentials. The platform-wide service account
-- (GOOGLE_CREDENTIALS_BASE64) remains the fallback for accounts without
-- a row here.
CREATE TABLE IF NOT EXISTS google_calendar_integrations (
    account_id              CHAR(26) PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    refresh_token_encrypted TEXT NOT NULL,
    google_email            TEXT NOT NULL DEFAULT '',
    connected_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
