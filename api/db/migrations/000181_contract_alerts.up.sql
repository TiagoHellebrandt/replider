-- Clause alerts and the recovered-revenue ledger (§6.11, G5/G6).
--
-- A red flag nobody scheduled is a fact nobody acts on. These two tables are
-- what turn "este contrato renova sozinho se você não avisar até 30/01" into a
-- task with a date, and then into a number: how much contracted revenue this
-- account actually went back and collected.

CREATE TABLE IF NOT EXISTS contract_alerts (
    id         CHAR(26)    PRIMARY KEY,
    account_id CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    -- CASCADE: an alert about a contract that no longer exists is a task nobody
    -- can complete.
    contract_id CHAR(26)   NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,

    kind TEXT NOT NULL CHECK (kind IN (
        -- The window to say "não renove" before it renews itself.
        'renewal_notice',
        -- The month the price is indexed.
        'adjustment',
        -- The term ending, whether it renews or not.
        'expiry',
        -- A multa worth reading before it is invoked.
        'penalty',
        -- Exclusivity, which constrains what the sales floor may promise.
        'exclusivity',
        -- An ata with balance left to deliver against.
        'registry_balance',
        'custom'
    )),

    title  TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',

    -- The day somebody has to act. DATE, not timestamp: these are calendar
    -- deadlines and an hour component only creates off-by-one arguments.
    due_on DATE NOT NULL,

    -- MoneyAtStakeCents is what acting on it is worth — the accumulated
    -- adjustment, the contract's value, the ata's balance. Zero when the flag is
    -- a constraint rather than an amount (exclusivity).
    money_at_stake_cents BIGINT NOT NULL DEFAULT 0,

    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','done','dismissed')),
    done_at   TIMESTAMPTZ,
    done_note TEXT NOT NULL DEFAULT '',

    -- The lead this alert produced, when it became one (G6). No FK: a lead may
    -- be deleted and the alert still records that it fired.
    lead_id CHAR(26),

    -- When it was last surfaced (digest, push). Stops the same alert being
    -- pushed every hour of the day it comes due.
    last_notified_at TIMESTAMPTZ,

    created_by_user_id CHAR(26),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT contract_alert_done_stamp
      CHECK ((status = 'pending' AND done_at IS NULL)
             OR (status <> 'pending' AND done_at IS NOT NULL))
);

-- The "what needs me" read: one account, pending, soonest first.
CREATE INDEX IF NOT EXISTS idx_contract_alerts_due
    ON contract_alerts (account_id, status, due_on);

CREATE INDEX IF NOT EXISTS idx_contract_alerts_contract
    ON contract_alerts (contract_id, status);

-- One PENDING alert per (contract, kind, date). The 1-click create and the
-- worker both write here, and without this the same renewal window would
-- produce a new task on every sweep. Partial, so a completed alert never blocks
-- next year's.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_contract_alert_pending
    ON contract_alerts (contract_id, kind, due_on)
    WHERE status = 'pending';

-- What the account actually went back and collected.
--
-- This is the number that renews the SaaS: "você recuperou R$418.000 em receita
-- contratada este ano". It is a ledger rather than a computed figure because the
-- amount is what a HUMAN says was collected — the accumulated index is what was
-- owed, and the two differ every time somebody negotiates.
CREATE TABLE IF NOT EXISTS contract_recoveries (
    id          CHAR(26)    PRIMARY KEY,
    account_id  CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    contract_id CHAR(26)    NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    -- The alert this came from, when it came from one. Nullable: somebody may
    -- record a recovery they found on their own.
    alert_id CHAR(26) REFERENCES contract_alerts(id) ON DELETE SET NULL,

    kind TEXT NOT NULL CHECK (kind IN ('adjustment','renewal','registry','other')),

    -- Positive only. A recovery is money collected; a loss is not a negative
    -- recovery, it is the absence of one, and allowing negatives here would let
    -- the headline number be argued down by bookkeeping.
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    note TEXT NOT NULL DEFAULT '',

    recorded_by_user_id CHAR(26),
    -- The date the money was recognised, which is what a period report groups
    -- by — not when the row happened to be typed.
    recognised_on DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_contract_recoveries_period
    ON contract_recoveries (account_id, recognised_on DESC);

CREATE INDEX IF NOT EXISTS idx_contract_recoveries_contract
    ON contract_recoveries (contract_id);
