-- Broadcast campaigns: one-to-many message dispatches via WhatsApp templates
-- (or future channels). Decoupled from the channel/conversation model so the
-- inbox is not polluted by mass sends — each delivery is tracked in its own
-- recipient row with an explicit status lifecycle.

CREATE TABLE campaigns (
    id              TEXT PRIMARY KEY,
    account_id      TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    channel_id      TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    template_name   TEXT NOT NULL,
    template_lang   TEXT NOT NULL DEFAULT 'pt_BR',
    -- Variable values to substitute into the template body. Stored as a JSON
    -- array of strings (positional, matching {{1}}, {{2}} in the template).
    -- Per-recipient personalization can override at send time.
    template_vars   JSONB NOT NULL DEFAULT '[]'::jsonb,
    status          TEXT NOT NULL DEFAULT 'draft', -- draft | scheduled | sending | done | failed | canceled
    scheduled_at    TIMESTAMPTZ,                   -- NULL = "send now" once status becomes scheduled
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    -- Aggregated counters maintained by the worker and the webhook handler.
    total_recipients INT NOT NULL DEFAULT 0,
    sent_count       INT NOT NULL DEFAULT 0,
    delivered_count  INT NOT NULL DEFAULT 0,
    read_count       INT NOT NULL DEFAULT 0,
    failed_count     INT NOT NULL DEFAULT 0,
    created_by       TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('draft','scheduled','sending','done','failed','canceled'))
);

CREATE INDEX idx_campaigns_account_status ON campaigns(account_id, status);
CREATE INDEX idx_campaigns_scheduled_at ON campaigns(scheduled_at) WHERE status = 'scheduled';

CREATE TABLE campaign_recipients (
    id              TEXT PRIMARY KEY,
    campaign_id     TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    -- The contact identifier on the channel (phone number for WhatsApp,
    -- chat_id for Telegram, etc.). We store as raw string to support all
    -- channel types without joining contacts on read.
    recipient       TEXT NOT NULL,
    -- Optional per-recipient variable overrides; merged on top of
    -- campaigns.template_vars at send time.
    vars            JSONB NOT NULL DEFAULT '[]'::jsonb,
    status          TEXT NOT NULL DEFAULT 'pending', -- pending | sent | delivered | read | failed
    -- Provider-side message id (e.g. WhatsApp wamid) used to correlate
    -- delivery/read status updates from the inbound webhook.
    provider_msg_id TEXT,
    error_message   TEXT,
    sent_at         TIMESTAMPTZ,
    delivered_at    TIMESTAMPTZ,
    read_at         TIMESTAMPTZ,
    failed_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(campaign_id, recipient),
    CHECK (status IN ('pending','sent','delivered','read','failed'))
);

CREATE INDEX idx_campaign_recipients_campaign_status ON campaign_recipients(campaign_id, status);
CREATE INDEX idx_campaign_recipients_provider_msg_id ON campaign_recipients(provider_msg_id) WHERE provider_msg_id IS NOT NULL;
