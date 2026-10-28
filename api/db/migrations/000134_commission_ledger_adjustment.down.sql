-- Rows with the new reason would violate the restored constraints.
DELETE FROM commission_ledger WHERE reason = 'adjustment';

ALTER TABLE commission_ledger DROP CONSTRAINT IF EXISTS commission_ledger_amount_cents_check;
ALTER TABLE commission_ledger ADD CONSTRAINT commission_ledger_amount_cents_check
    CHECK (amount_cents >= 0);

ALTER TABLE commission_ledger DROP CONSTRAINT IF EXISTS commission_ledger_reason_check;
ALTER TABLE commission_ledger ADD CONSTRAINT commission_ledger_reason_check
    CHECK (reason IN ('standard','residual','sla_reduced','bonus_upgrade','bonus_nps','bonus_retention'));
