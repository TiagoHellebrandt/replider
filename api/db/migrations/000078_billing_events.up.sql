-- Billing events ledger for revenue analytics (US-071). Today MRR/LTV/
-- churn are uncomputable because the only billing signals we keep are
-- the mutable account.subscription_status + a transient
-- processed_stripe_events claim row. This append-only ledger captures
-- one immutable row per revenue-affecting Stripe event so the admin
-- dashboard can derive MRR (sum of the latest non-churn amount per
-- account), churn (count of churn events in a window) and LTV
-- (ARPA / churn rate).
--
-- amount_cents is the recurring monthly amount for the account at the
-- time of the event (0 for churn). plan is the plan name snapshot so
-- historical rows survive a plan rename. occurred_at is the Stripe
-- event timestamp; created_at is when we wrote the row.
CREATE TABLE billing_events (
    id           CHAR(26) PRIMARY KEY,
    account_id   CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    event_type   TEXT NOT NULL,           -- subscription_created | subscription_renewed | churned
    plan         TEXT NOT NULL DEFAULT '',
    amount_cents BIGINT NOT NULL DEFAULT 0,
    currency     TEXT NOT NULL DEFAULT '',
    occurred_at  TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- "latest event per account" drives MRR: ORDER BY occurred_at DESC per
-- account, so account_id + occurred_at is the workhorse index.
CREATE INDEX idx_billing_events_account_occurred
    ON billing_events(account_id, occurred_at DESC);

-- Churn-window scans filter by event_type + occurred_at across all
-- accounts.
CREATE INDEX idx_billing_events_type_occurred
    ON billing_events(event_type, occurred_at);
