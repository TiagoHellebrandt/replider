-- Revert TOTP secret encryption (US-076). Restores the plaintext Base32
-- column; sealed secrets are not recoverable here (no KEK in SQL), so any
-- staged secret is cleared and the user re-runs Setup.
ALTER TABLE users
    ADD COLUMN totp_secret VARCHAR(255);

ALTER TABLE users
    DROP COLUMN totp_secret_encrypted;
