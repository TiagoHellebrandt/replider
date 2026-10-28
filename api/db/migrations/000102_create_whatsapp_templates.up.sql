-- Templates WhatsApp (US-018): local cache of a channel's Meta-approved
-- message templates. WhatsApp requires pre-approved templates to open a
-- conversation outside the 24h customer-service window (outbound sales).
-- We sync them from the Graph API (message_templates) into this table so
-- the dashboard / send pipeline can render + validate without a Graph
-- roundtrip on every interaction.
--
-- account_id + channel_id cascade-delete with their parents. The
-- UNIQUE(channel_id, name, language) pair makes the sync idempotent:
-- re-syncing upserts the existing row instead of duplicating it (Meta
-- allows the same template name across multiple languages).
--
-- `components` is the verbatim Meta components array (header/body/footer/
-- buttons); `body` is the flattened BODY text we extract for previews;
-- `variables` is the JSONB array of positional placeholders ({{1}}, {{2}}…)
-- parsed out of the body so the send adapter (US-019) knows how many
-- parameters to bind.
CREATE TABLE IF NOT EXISTS whatsapp_templates (
    id               TEXT PRIMARY KEY,
    account_id       TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    channel_id       TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    meta_template_id TEXT NOT NULL DEFAULT '',
    name             TEXT NOT NULL,
    language         TEXT NOT NULL,
    category         TEXT NOT NULL DEFAULT '',
    meta_status      TEXT NOT NULL DEFAULT '',
    body             TEXT NOT NULL DEFAULT '',
    components       JSONB NOT NULL DEFAULT '[]'::jsonb,
    variables        JSONB NOT NULL DEFAULT '[]'::jsonb,
    rejected_reason  TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (channel_id, name, language)
);

CREATE INDEX IF NOT EXISTS idx_whatsapp_templates_account ON whatsapp_templates(account_id);
CREATE INDEX IF NOT EXISTS idx_whatsapp_templates_channel ON whatsapp_templates(channel_id);
