-- US-008 (review fixes): a commission_ledger row — above all a PAID one
-- — is settled money and audit trail. The original FKs were ON DELETE
-- CASCADE, so a GDPR account deletion (or removing a seller user)
-- silently erased paid commission history, destroying the payout trail.
--
-- The ledger now keeps seller_user_id/account_id as an immutable
-- SNAPSHOT: the FKs are dropped, rows survive the referenced rows'
-- deletion. RESTRICT was rejected because it would block the GDPR
-- deletion itself (a legal requirement); retaining the pseudonymous ids
-- for financial reconciliation is a legitimate-interest basis. Every
-- read query already LEFT JOINs users/accounts, so listings degrade to
-- empty names instead of dropping rows.
ALTER TABLE commission_ledger DROP CONSTRAINT IF EXISTS commission_ledger_seller_user_id_fkey;
ALTER TABLE commission_ledger DROP CONSTRAINT IF EXISTS commission_ledger_account_id_fkey;

-- SumPaidProfitShare/DeleteStaleProfitShare filter on (account, cycle);
-- with the FK gone this index also stands in for the FK's lookup use.
CREATE INDEX IF NOT EXISTS idx_commission_ledger_account
    ON commission_ledger (account_id, cycle);
