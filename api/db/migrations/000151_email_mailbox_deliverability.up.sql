-- US-M007: per-mailbox sending caps and warmup state.
--
-- The failure this prevents is permanent and unrecoverable by us: a new
-- domain that starts sending 500 cold emails on day one gets its reputation
-- burned at Gmail/Outlook, and no amount of later good behaviour fully
-- undoes it. The customer's domain is the asset at risk, and they usually
-- do not know the rules — so the platform enforces them rather than
-- documenting them.
--
-- One row per SENDING MAILBOX (a channel's from_address). Not per account:
-- reputation is tracked by mailbox and domain at the receiving end, so the
-- cap has to live at the same granularity.
CREATE TABLE IF NOT EXISTS email_mailboxes (
    id          CHAR(26) PRIMARY KEY,
    account_id  CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    channel_id  TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    -- Lowercased address; the domain is derived, not stored twice.
    address     TEXT NOT NULL,
    domain      TEXT NOT NULL,

    -- Daily ceiling. 25/day/mailbox is the industry-standard cold-outreach
    -- cap; it is configurable because a warmed mailbox on a mature domain
    -- can carry more, and because a customer with a real deliverability
    -- consultant should not be argued with.
    daily_cap   INT NOT NULL DEFAULT 25,

    -- Warmup: NULL start means "not warming" (an established mailbox the
    -- customer already sends from). A warming mailbox has a ceiling that
    -- ramps over ~3 weeks, computed from this date.
    warmup_started_at TIMESTAMPTZ,
    -- Set when the operator/customer declares the mailbox established.
    warmup_completed_at TIMESTAMPTZ,

    -- Rolling counter, reset by the sender when the UTC day changes.
    sent_today  INT NOT NULL DEFAULT 0,
    sent_date   DATE,

    -- Domain authentication (US-M006). Cold sending is blocked until this
    -- is stamped; it is set by a successful DNS verification.
    dns_verified_at TIMESTAMPTZ,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One mailbox row per address per account: two channels sharing an address
-- share the cap, which is correct — the receiving end sees one sender.
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_mailboxes_account_address
    ON email_mailboxes(account_id, address);
CREATE INDEX IF NOT EXISTS idx_email_mailboxes_channel
    ON email_mailboxes(channel_id);
-- Domain-level reporting ("how much did we send from clinica.com.br today?")
CREATE INDEX IF NOT EXISTS idx_email_mailboxes_domain
    ON email_mailboxes(account_id, domain);
