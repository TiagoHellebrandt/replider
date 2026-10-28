-- Senders a human has vouched for (§6.10, camada 3).
--
-- A false positive that cannot be dismissed is worse than no warning: the
-- reader learns to scroll past the banner, and then the real one is invisible
-- too. One click, and this account stops being warned about that sender.
--
-- Deliberately per-account. Vouching is a statement about a relationship
-- ("this is my supplier"), not about a domain in general, and one customer's
-- trust must never silence a warning for another.
CREATE TABLE IF NOT EXISTS sender_allowlist (
    account_id  CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    address     TEXT        NOT NULL,
    -- Who vouched, for the audit trail: this is a security decision and it
    -- has an author.
    allowed_by  CHAR(26),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, address)
);
