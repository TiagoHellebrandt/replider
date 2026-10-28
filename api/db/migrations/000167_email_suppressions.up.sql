-- Addresses this account must not write to (§6.8.4).
--
-- Separate from contacts.opted_out_at on purpose: an opt-out is the PERSON
-- saying stop, a suppression is the MAILBOX saying stop. They have different
-- causes, different appeals, and a bounce must suppress even for an address
-- that was never a contact (a typo'd import, say).
CREATE TABLE IF NOT EXISTS email_suppressions (
    id         CHAR(26)    PRIMARY KEY,
    account_id CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    address    TEXT        NOT NULL,
    reason     TEXT        NOT NULL CHECK (reason IN ('hard_bounce', 'complaint', 'manual')),
    diagnostic TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Case-insensitive uniqueness: Dono@Clinica.com and dono@clinica.com are the
-- same mailbox, and suppressing one while still writing to the other would
-- make the whole list useless.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_email_suppressions_account_address
    ON email_suppressions (account_id, LOWER(address));
