-- US-004 (review fixes): paid ledger rows are immutable. A recompute of
-- an (account, cycle) whose profit share already settled must never
-- mint a second full pending row (a reason flip or a seller
-- reallocation would pay the same profit twice) — the difference vs the
-- paid total is recorded as an 'adjustment' entry instead. The delta
-- can be negative (the computed value dropped after an early payout),
-- so the >= 0 rail applies to every reason but 'adjustment'.
ALTER TABLE commission_ledger DROP CONSTRAINT IF EXISTS commission_ledger_reason_check;
ALTER TABLE commission_ledger ADD CONSTRAINT commission_ledger_reason_check
    CHECK (reason IN ('standard','residual','sla_reduced','bonus_upgrade','bonus_nps','bonus_retention','adjustment'));

ALTER TABLE commission_ledger DROP CONSTRAINT IF EXISTS commission_ledger_amount_cents_check;
ALTER TABLE commission_ledger ADD CONSTRAINT commission_ledger_amount_cents_check
    CHECK (amount_cents >= 0 OR reason = 'adjustment');
