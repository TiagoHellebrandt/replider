-- The anti-scam flywheel (§6.10, camada 4).
--
-- Scams evolve; the system is never "done". The loop is: a human reports one,
-- the platform extracts its signature, and the signature protects everyone
-- else. That is the network effect the planning document calls the defensive
-- moat — more customers means faster reaction to a new scam.

CREATE TABLE IF NOT EXISTS scam_reports (
    id            CHAR(26)    PRIMARY KEY,
    account_id    CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    -- The message stays referenced for the reporter's own audit, but the
    -- SIGNATURE below is what crosses account boundaries.
    message_id    CHAR(26)    REFERENCES messages(id) ON DELETE SET NULL,
    reported_by   CHAR(26),

    -- The indicator itself. A scammer's domain is not sensitive customer
    -- data, so this crosses accounts with no k-anonymity — unlike the
    -- anonymised strategy tier, where the data IS about a customer.
    sender_domain TEXT        NOT NULL,
    -- Bank details, boleto lines and CNPJs the scam used, so the same
    -- account reappearing under a new domain is still caught.
    payment_values TEXT[]     NOT NULL DEFAULT '{}',

    -- The reporter's own words. This is the highest-value column in the
    -- table: "esse fornecedor nunca manda boleto por email" is knowledge no
    -- technical check has, and it becomes few-shot for the classifier.
    reason        TEXT        NOT NULL DEFAULT '',

    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One report per account per domain. A single customer cannot push a
    -- domain over the network threshold by reporting it repeatedly.
    UNIQUE (account_id, sender_domain)
);

-- The question asked on every inbound: how many DIFFERENT accounts reported
-- this domain?
CREATE INDEX IF NOT EXISTS idx_scam_reports_domain
    ON scam_reports (sender_domain);

-- Same question for a payment identifier reappearing under a new domain.
CREATE INDEX IF NOT EXISTS idx_scam_reports_payment_values
    ON scam_reports USING GIN (payment_values);
