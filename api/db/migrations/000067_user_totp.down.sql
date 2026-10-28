ALTER TABLE users
    DROP COLUMN IF EXISTS totp_secret,
    DROP COLUMN IF EXISTS totp_enabled,
    DROP COLUMN IF EXISTS totp_recovery_codes,
    DROP COLUMN IF EXISTS totp_verified_at;
