-- PIX renewal + dunning (US-M014).
--
-- PIX does not auto-renew: there is no card on file to charge. A
-- subscription paid by PIX is really "we ask again every month", so the
-- platform has to generate the charge, chase it, and eventually stop the
-- service — none of which Stripe was doing for us on this path.
--
-- Stage bookkeeping mirrors the reactivation ladder (000092): one SMALLINT
-- of "furthest stage already sent", advanced with GREATEST, plus the
-- period it belongs to so a renewed subscription starts a fresh ladder
-- instead of inheriting last month's progress.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS pix_cycle_stage_sent SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS pix_cycle_period_end TIMESTAMPTZ,
    -- The charge currently outstanding, so a reminder can point at the
    -- same PIX code instead of minting a second one the customer might
    -- also pay.
    ADD COLUMN IF NOT EXISTS pix_cycle_charge_id VARCHAR(64);

CREATE INDEX IF NOT EXISTS idx_accounts_pix_cycle_due
    ON accounts (current_period_end)
    WHERE current_period_end IS NOT NULL;
