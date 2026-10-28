DROP INDEX IF EXISTS uniq_commission_ledger_pending;
-- Restoring the table-level UNIQUE requires collapsing the settled
-- duplicates the partial index allowed (several paid adjustments per
-- (account, cycle)); keep the newest row of each tuple.
DELETE FROM commission_ledger l
 USING commission_ledger d
 WHERE l.seller_user_id = d.seller_user_id
   AND l.account_id = d.account_id
   AND l.cycle = d.cycle
   AND l.reason = d.reason
   AND l.id < d.id;
ALTER TABLE commission_ledger
    ADD CONSTRAINT commission_ledger_seller_user_id_account_id_cycle_reason_key
    UNIQUE (seller_user_id, account_id, cycle, reason);
