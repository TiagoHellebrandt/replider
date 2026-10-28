-- TOTP (RFC 6238) two-factor authentication for owner-panel super-admins.
-- The flow is opt-in for now: any user can enroll, but only super-admins
-- have a routing reason to do so until phase 2 lands the login MFA pivot
-- and the owner middleware enforcement.
--
--   totp_secret           Base32-encoded shared secret. Stored as VARCHAR
--                         instead of bytea so we can render it back to the
--                         user during enrollment without an encode/decode
--                         dance, and rotate via TRUNCATE+UPDATE if a
--                         secret leaks.
--   totp_enabled          Set true only after the user proves possession
--                         by entering a valid 6-digit code (Verify call).
--   totp_recovery_codes   bcrypt'd one-time recovery codes (10 codes, 8
--                         chars each). Plain text shown ONCE at enrollment;
--                         compared with constant-time bcrypt on use.
--   totp_verified_at      Last successful 2FA proof — phase 2 reads this
--                         to decide whether to re-prompt on /owner.
ALTER TABLE users
    ADD COLUMN totp_secret         VARCHAR(255),
    ADD COLUMN totp_enabled        BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN totp_recovery_codes TEXT[],
    ADD COLUMN totp_verified_at    TIMESTAMPTZ;
