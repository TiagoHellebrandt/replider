-- Durable journal of conversation-overage charges (the C2 fix from
-- docs/OVERAGE_METERING_DESIGN.md). Each row is one Stripe invoice item
-- the overage settler creates.
--
-- Why it exists: a crash between the Stripe call and the local commit
-- used to leave the running tally behind, so the next tick recomputed
-- the same delta and — once Stripe's 24h idempotency key expired —
-- billed it twice. This journal is the single source of truth for
-- "what has been charged this cycle": the settler resumes an in-flight
-- `pending` row instead of recomputing, and escalates a row stuck
-- beyond the idempotency window to `needs_review` rather than risking
-- a duplicate charge.
--
-- status: pending | charged | archived | needs_review
--   pending      — journalled, Stripe call may or may not have landed
--   charged      — Stripe confirmed; counts toward the cycle total
--   archived     — cycle closed on invoice.paid; no longer counted
--   needs_review — stuck past the idempotency window; human resolves
CREATE TABLE IF NOT EXISTS overage_charges (
    id                          CHAR(26)    PRIMARY KEY,
    account_id                  CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    amount_cents                INT         NOT NULL,
    running_total_cents         INT         NOT NULL,
    overage_conversations_count INT         NOT NULL,
    capped                      BOOLEAN     NOT NULL DEFAULT FALSE,
    status                      VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    charged_at                  TIMESTAMPTZ
);

-- At most one unresolved charge per account — the settler resumes the
-- pending row before journalling a new one.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_overage_charge_pending_per_account
    ON overage_charges (account_id) WHERE status = 'pending';

-- SumCharged / ArchiveCharged scan per-account by status each tick.
CREATE INDEX IF NOT EXISTS idx_overage_charges_account_status
    ON overage_charges (account_id, status);
