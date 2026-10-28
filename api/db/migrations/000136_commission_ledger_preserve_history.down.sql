-- Rollback: re-adding the CASCADE FKs requires purging ledger rows that
-- outlived their account/user (the snapshot semantics the up migration
-- introduced) — they would violate the constraints.
DELETE FROM commission_ledger l
 WHERE NOT EXISTS (SELECT 1 FROM accounts a WHERE a.id = l.account_id)
    OR NOT EXISTS (SELECT 1 FROM users u WHERE u.id = l.seller_user_id);

ALTER TABLE commission_ledger
    ADD CONSTRAINT commission_ledger_seller_user_id_fkey
    FOREIGN KEY (seller_user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE commission_ledger
    ADD CONSTRAINT commission_ledger_account_id_fkey
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_commission_ledger_account;
