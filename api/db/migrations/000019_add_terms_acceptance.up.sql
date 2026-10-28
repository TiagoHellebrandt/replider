-- LGPD Art. 8 / Civil Code Art. 425: the user must explicitly accept the
-- Terms of Service and Privacy Policy before signing the contract that
-- creates the account. We persist (a) WHEN they accepted and (b) WHICH
-- version they accepted, so that future term updates can correctly require
-- a fresh acceptance from existing users.
ALTER TABLE users
    ADD COLUMN terms_accepted_at TIMESTAMPTZ,
    ADD COLUMN terms_version     TEXT;

-- Backfill: existing users (created before this migration) are recorded as
-- having accepted v0 ("legacy") on the migration timestamp. This is a
-- pragmatic compromise — they DID accept whatever terms were in effect at
-- registration time, but those weren't versioned. New term updates will
-- force them to re-accept since terms_version != current.
UPDATE users
SET terms_accepted_at = COALESCE(created_at, NOW()),
    terms_version     = 'v0-legacy'
WHERE terms_accepted_at IS NULL;
