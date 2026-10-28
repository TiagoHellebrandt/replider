-- 2026-09-01 hardening: password-reset and e-mail-verification tokens are
-- now stored as SHA-256 hashes (domain.HashToken) and looked up by hash, so
-- a read of the users table no longer hands out live reset links.
--
-- Existing clear tokens cannot be converted in place without also knowing
-- the hash of a value we are about to stop trusting, and they expire within
-- 24h anyway: they are cleared. A user mid-flow requests a new mail
-- (forgot-password / resend-verification); nothing else is affected.
UPDATE users
SET reset_token = NULL,
    reset_token_expires_at = NULL,
    verification_token = NULL,
    verification_token_expires_at = NULL
WHERE reset_token IS NOT NULL OR verification_token IS NOT NULL;
