-- Encrypt TOTP shared secrets at rest (US-076).
--
-- Migration 000067 stored `totp_secret` as a Base32 VARCHAR in plaintext.
-- A DB dump / read-replica leak therefore exposed every enrolled user's
-- TOTP seed, which is sufficient to mint valid codes forever. We move the
-- seed into a `bytea` column that holds AES-256-GCM ciphertext
-- (`nonce || ciphertext || tag`) sealed by the application with a KEK
-- loaded from the `TOTP_KEK` env var.
--
-- Backfill note: AES-GCM sealing cannot run in pure SQL — the KEK lives in
-- the application process, never in Postgres — so we cannot transform the
-- existing plaintext into ciphertext here. TOTP enrollment is opt-in
-- super-admin scaffolding (the phase-2 login gate has not shipped), so in
-- practice no row carries a live secret. Rather than copy the Base32
-- plaintext verbatim into a column named `_encrypted` (which would defeat
-- the entire point of this story), we clear any staged secret. The handful
-- of users who may have staged an unverified secret simply re-run Setup;
-- since `totp_enabled` only ever flips true *after* a verified code, no one
-- is locked out.
ALTER TABLE users
    ADD COLUMN totp_secret_encrypted bytea;

-- Drop the plaintext column. Any staged-but-unverified secret is discarded
-- (see backfill note above); enrolled users keep `totp_enabled` /
-- `totp_recovery_codes` and re-stage a fresh sealed secret via Setup.
-- ALLOW-DESTRUCTIVE: the entire purpose of this migration is to stop storing
-- the TOTP seed in plaintext; the staged-secret discard is documented above.
ALTER TABLE users
    DROP COLUMN totp_secret;
