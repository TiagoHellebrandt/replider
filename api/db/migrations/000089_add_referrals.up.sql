-- US-174: Referral program.
-- Each account gets a shareable referral_code (generated lazily on first
-- view, so existing rows stay NULL until used). A new signup that arrives
-- with ?ref=CODE records who referred it (referred_by_account_id) plus a
-- row in `referrals` that tracks the "1 month free for both" reward state.

ALTER TABLE accounts ADD COLUMN IF NOT EXISTS referral_code TEXT;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS referred_by_account_id TEXT
    REFERENCES accounts(id) ON DELETE SET NULL;

-- Partial unique index: many NULLs allowed (codes are minted lazily),
-- but every minted code is globally unique so ?ref=CODE resolves to one
-- account.
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_referral_code
    ON accounts(referral_code) WHERE referral_code IS NOT NULL;

CREATE TABLE IF NOT EXISTS referrals (
    id                  TEXT PRIMARY KEY,
    referrer_account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    referee_account_id  TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    -- pending  → referee signed up via a referral link, not yet paid
    -- rewarded → referee paid for the first time, both sides credited
    status              TEXT NOT NULL DEFAULT 'pending',
    reward_amount_cents BIGINT NOT NULL DEFAULT 0,
    reward_currency     TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    rewarded_at         TIMESTAMPTZ,
    -- A referee can only ever be referred once, so the reward fires once.
    CONSTRAINT uq_referrals_referee UNIQUE (referee_account_id)
);

CREATE INDEX IF NOT EXISTS idx_referrals_referrer
    ON referrals(referrer_account_id);
