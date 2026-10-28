-- US-002 (vendedores v3): link a customer account to the seller (a user
-- of the Replider platform account) who owns it. seller_user_id is the
-- carteira source of truth; lead_source decides the commission track
-- (seller_ref = 50%, inbound = 20%). ON DELETE SET NULL so removing a
-- seller user never cascades into customer accounts.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS seller_user_id CHAR(26) NULL REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS lead_source TEXT NULL CHECK (lead_source IN ('seller_ref', 'inbound')),
    ADD COLUMN IF NOT EXISTS seller_linked_at TIMESTAMPTZ NULL;

-- Carteira lookups are always "accounts of seller X" — partial index
-- keeps it cheap without bloating the unlinked majority.
CREATE INDEX IF NOT EXISTS idx_accounts_seller_user_id
    ON accounts (seller_user_id)
    WHERE seller_user_id IS NOT NULL;
