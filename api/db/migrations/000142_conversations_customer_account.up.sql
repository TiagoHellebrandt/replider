-- US-002 (canal-interno-cs): internal CS threads live on the PLATFORM
-- account's internal channel, but each thread belongs to a user of a
-- CUSTOMER account. customer_account_id stamps that account on the
-- conversation so the seller panel can group threads by carteira account
-- and SLA can aggregate per account. NULL for every non-internal
-- conversation (and for legacy rows).
ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS customer_account_id CHAR(26) NULL
        REFERENCES accounts(id) ON DELETE SET NULL;

-- Partial index: only internal threads carry the stamp, so the index
-- stays tiny while serving the seller-panel grouping lookups.
CREATE INDEX IF NOT EXISTS idx_conversations_customer_account
    ON conversations (customer_account_id)
    WHERE customer_account_id IS NOT NULL;
