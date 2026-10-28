-- WhatsApp Flows: native interactive forms rendered inside WhatsApp.
-- Stored locally so the dashboard can list/select flows without a
-- round-trip to Meta on every render. The flow_id from Meta is the
-- canonical identifier used in Cloud API send payloads; we cache the
-- json schema + status so the picker can show name, status, and a
-- preview without re-fetching.
--
-- Account-scoped. A nullable account_id reserves space for future
-- "global" / Replider-curated flows (account_id IS NULL means anyone
-- can use it). For now every row is per-account; the listing query
-- filters with `(account_id = $1 OR account_id IS NULL)` so the
-- global slot just works the day we flip it on.
CREATE TABLE whatsapp_flows (
    id              CHAR(26) PRIMARY KEY,                                       -- ULID
    account_id      CHAR(26) REFERENCES accounts(id) ON DELETE CASCADE,         -- nullable: NULL = global/curated
    channel_id      CHAR(26) REFERENCES channels(id) ON DELETE SET NULL,        -- which WABA this flow lives in (nullable for global)
    waba_id         TEXT,                                                       -- denormalized so we can call Graph API after channel deletion
    flow_id         TEXT NOT NULL,                                              -- Meta's canonical flow id (numeric string)
    name            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'DRAFT',                              -- DRAFT | PUBLISHED | DEPRECATED | BLOCKED | THROTTLED
    categories      JSONB NOT NULL DEFAULT '[]'::jsonb,                         -- e.g. ["SIGN_UP","LEAD_GENERATION","APPOINTMENT_BOOKING"]
    flow_json       JSONB,                                                      -- full flow JSON schema (screens, components, actions); null until first sync
    preview_url     TEXT,                                                       -- Meta's web preview URL (when PUBLISHED)
    validation_errors JSONB NOT NULL DEFAULT '[]'::jsonb,                       -- last validation errors from Meta (empty when valid)
    -- endpoint_uri is the URL Meta calls during data exchange for
    -- dynamic flows. NULL means the flow is fully static (no server
    -- callbacks) — most lead/booking flows fall in this bucket.
    endpoint_uri    TEXT,
    -- is_dynamic mirrors endpoint_uri's presence so the picker can
    -- warn the operator that publishing requires a working data
    -- exchange endpoint + RSA keypair on the server.
    is_dynamic      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Listing query: per-account flows + globals, newest first.
CREATE INDEX idx_whatsapp_flows_account ON whatsapp_flows(account_id, created_at DESC);

-- Sync worker / webhook handler looks up by Meta's flow_id when a
-- status update arrives. Unique per WABA because Meta's flow_id is
-- only unique within a WABA, not globally across our tenants.
CREATE UNIQUE INDEX uniq_whatsapp_flows_waba_flowid ON whatsapp_flows(waba_id, flow_id) WHERE waba_id IS NOT NULL;

-- The data-exchange endpoint receives signed payloads; per-flow
-- lookup by flow_id is the hot path. Partial index on PUBLISHED so
-- DRAFT/DEPRECATED don't bloat the lookup tree.
CREATE INDEX idx_whatsapp_flows_published ON whatsapp_flows(flow_id) WHERE status = 'PUBLISHED';
