-- Payment details a contact has used before (§6.10, camada 2).
--
-- The golpe do boleto is a hijacked thread carrying different bank details.
-- Nothing about that message is spammy, so no generic filter sees it. What
-- catches it is knowing what THAT contact's details have always been — which
-- only a system holding the whole relationship knows. This table is that
-- memory.

CREATE TABLE IF NOT EXISTS contact_payment_details (
    account_id   CHAR(26)     NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    -- The contact as the pipeline knows them (external_id: the email
    -- address for the email channel).
    contact_key  TEXT         NOT NULL,

    kind         TEXT         NOT NULL
        CHECK (kind IN ('pix_key', 'bank_account', 'boleto', 'cnpj')),
    -- Normalised for comparison: "1234-5" and "12345" are one account.
    value        TEXT         NOT NULL,
    -- What actually appeared, for showing the reader. "Ag 1234 / CC 56789-0"
    -- is recognisable; "1234567890" is not.
    raw          TEXT         NOT NULL DEFAULT '',

    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    times_seen    INTEGER     NOT NULL DEFAULT 1,

    PRIMARY KEY (account_id, contact_key, kind, value)
);

-- The read: everything this contact has ever used, on the inbound hot path.
CREATE INDEX IF NOT EXISTS idx_contact_payment_details_lookup
    ON contact_payment_details (account_id, contact_key);
