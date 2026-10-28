-- Approvals past the mailbox's daily cap stop being refusals and become
-- scheduled sends (Ofertas → Hoje, phase 1b). The row is the decision the
-- human already made ("send this"); the worker executes it when the warmup
-- budget renews, through the same ApproveSend gates as a live click.
CREATE TABLE IF NOT EXISTS approach_schedule (
    id          CHAR(26) PRIMARY KEY,
    account_id  CHAR(26) NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    lead_id     CHAR(26) NOT NULL REFERENCES leads (id) ON DELETE CASCADE,
    -- The email subject frozen at approval time. The BODY is deliberately
    -- NOT frozen: the worker sends the lead's current approach_draft, so an
    -- edit made while the send waits goes out instead of a stale copy.
    subject     TEXT NOT NULL DEFAULT '',
    send_at     TIMESTAMPTZ NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending', -- pending | processing | sent | failed
    attempts    INT NOT NULL DEFAULT 0,
    last_error  TEXT NOT NULL DEFAULT '',
    sent_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One live schedule per lead: re-approving an already-scheduled lead
-- refreshes the row instead of queuing a second send.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_approach_schedule_live
    ON approach_schedule (lead_id)
    WHERE status IN ('pending', 'processing');

CREATE INDEX IF NOT EXISTS idx_approach_schedule_due
    ON approach_schedule (send_at)
    WHERE status = 'pending';
