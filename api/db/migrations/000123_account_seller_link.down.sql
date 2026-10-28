DROP INDEX IF EXISTS idx_accounts_seller_user_id;
ALTER TABLE accounts
    DROP COLUMN IF EXISTS seller_linked_at,
    DROP COLUMN IF EXISTS lead_source,
    DROP COLUMN IF EXISTS seller_user_id;
