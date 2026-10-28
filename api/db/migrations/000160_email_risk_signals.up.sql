-- Risk signals on inbound email (§6.10, camada 1).
--
-- One row per message that had ANYTHING found. A message with nothing is
-- simply absent: storing "we looked and found nothing" for every inbound
-- would double the write volume of the busiest table in the product to
-- record the normal case.

CREATE TABLE IF NOT EXISTS email_risk_assessments (
    message_id   CHAR(26)     PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    -- Denormalised for the same reason as attachments: every read is scoped
    -- by account, and a filter in the WHERE clause cannot be forgotten.
    account_id   CHAR(26)     NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    score        SMALLINT     NOT NULL DEFAULT 0,
    -- Held out of the normal flow, shown with a warning. Derived from the
    -- score at write time and STORED, because the threshold may move and a
    -- past decision should stay explainable.
    quarantined  BOOLEAN      NOT NULL DEFAULT FALSE,

    -- The signals with their evidence, verbatim as shown to the user. JSONB
    -- rather than a child table: it is read whole, never queried by element,
    -- and the shape belongs to the domain, not to the schema.
    signals      JSONB        NOT NULL DEFAULT '[]'::jsonb,

    -- The sender's domain, for the collective blocklist in camada 4 and for
    -- answering "quantas contas viram este domínio".
    sender_domain TEXT        NOT NULL DEFAULT '',

    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- The quarantine list, per account, newest first.
CREATE INDEX IF NOT EXISTS idx_email_risk_quarantined
    ON email_risk_assessments (account_id, created_at DESC)
    WHERE quarantined;

-- Feeds "this domain was reported by other companies on the network".
CREATE INDEX IF NOT EXISTS idx_email_risk_sender_domain
    ON email_risk_assessments (sender_domain)
    WHERE sender_domain <> '';
