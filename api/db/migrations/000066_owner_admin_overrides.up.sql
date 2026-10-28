-- Owner admin: per-account suspend flag + audit reason.
--
-- Existing per-cap override columns (000060/000062) already cover the
-- "custom contract caps" need — sales records the negotiated number in
-- max_*_override and EffectiveMax* helpers merge it on top of the base
-- plan. This migration only adds the suspend gate the owner panel uses
-- to disable a misbehaving account without deleting it.
ALTER TABLE accounts
    ADD COLUMN suspended_at     TIMESTAMPTZ,
    ADD COLUMN suspended_reason TEXT;

-- Hot-path read: inbound webhooks check "is this account suspended"
-- on every message; partial index keeps the lookup at O(log live-suspended).
CREATE INDEX idx_accounts_suspended ON accounts(suspended_at)
    WHERE suspended_at IS NOT NULL;
