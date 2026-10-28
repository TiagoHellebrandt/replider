-- Outbound webhooks. Lets each tenant subscribe an external HTTPS
-- endpoint to events on their account (new inbound message,
-- conversation closed, etc.) without us coding a per-integration
-- adapter. The Zapier / n8n / Make / custom-CRM unlock — we emit a
-- signed POST and the consumer wires up the rest.
--
-- One row per (account, url) — duplicate URLs would just double-fire.
-- `events` is a TEXT[] of event names the webhook wants. Consumers
-- pick only what they need to keep their workers cheap.
--
-- failure_count auto-disables (is_active=false) the webhook when
-- consecutive failures exceed a threshold (handled by the dispatcher,
-- not the schema). last_succeeded_at lets the dashboard surface a
-- "last delivered Xh ago" health indicator.
CREATE TABLE account_webhooks (
    id                TEXT        PRIMARY KEY,
    account_id        TEXT        NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    url               TEXT        NOT NULL,
    secret            TEXT        NOT NULL,             -- HMAC-SHA256 secret minted on create
    events            TEXT[]      NOT NULL DEFAULT '{}',-- e.g. ['message.received','conversation.closed']
    is_active         BOOLEAN     NOT NULL DEFAULT true,
    description       TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_fired_at     TIMESTAMPTZ,
    last_succeeded_at TIMESTAMPTZ,
    last_error        TEXT,
    failure_count     INT         NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX idx_account_webhooks_account_url
    ON account_webhooks(account_id, url);

-- Hot path index: dispatcher fans out by account on every event,
-- filters to active rows that subscribe to the firing event name.
CREATE INDEX idx_account_webhooks_account_active
    ON account_webhooks(account_id)
    WHERE is_active = true;
