-- US-005 (review fixes): a settled adjustment must not block the NEXT
-- delta. The table-level UNIQUE (seller, account, cycle, reason) made a
-- new pending 'adjustment' insert conflict with an already-PAID
-- adjustment row for the same tuple — the ON CONFLICT ... pending guard
-- turned it into a silent no-op and the recompute's delta was lost.
-- Uniqueness only matters among PENDING rows (the recompute's working
-- set); paid rows are immutable history and may legitimately accumulate
-- several settled adjustments for one (account, cycle).
ALTER TABLE commission_ledger
    DROP CONSTRAINT IF EXISTS commission_ledger_seller_user_id_account_id_cycle_reason_key;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_commission_ledger_pending
    ON commission_ledger (seller_user_id, account_id, cycle, reason)
    WHERE status = 'pending';
