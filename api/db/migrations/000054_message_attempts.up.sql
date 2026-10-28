-- One outbound message can fan out into multiple sends per provider.
-- The bot reply pipeline splits an AI reply into 2-4 bubbles and
-- POSTs each as a separate WhatsApp message — every bubble gets its
-- own wamid, but until now the schema only stored one
-- external_message_id per row. Status updates (delivered/read/failed)
-- for bubbles 2..N had no row to correlate against and were silently
-- dropped, breaking the dashboard's checkmark timeline for every
-- multi-bubble reply.
--
-- message_attempts is a 1:N child of messages keyed on a single
-- (channel_id, wamid) pair so persistWhatsAppStatus can match either
-- the parent's external_message_id (legacy single-bubble path) or
-- one of the attempts (split path). Each row carries the per-attempt
-- timeline (sent/delivered/read/failed) independently of the parent.
CREATE TABLE message_attempts (
    id              CHAR(26) PRIMARY KEY,
    message_id      CHAR(26) NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    channel_id      CHAR(26) NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    -- The provider-side identifier (WhatsApp wamid). NULL when the
    -- send failed before Meta returned an id, so we still have a row
    -- to record the failure_reason.
    external_message_id TEXT,
    bubble_index    SMALLINT NOT NULL DEFAULT 0,
    sent_at         TIMESTAMPTZ,
    delivered_at    TIMESTAMPTZ,
    read_at         TIMESTAMPTZ,
    failed_at       TIMESTAMPTZ,
    failed_reason   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Lookup index for the inbound webhook handler matching a status
-- update back to its attempt. Partial because most attempts that
-- failed before Meta accepted have no wamid.
CREATE UNIQUE INDEX idx_message_attempts_external_id
    ON message_attempts(channel_id, external_message_id)
    WHERE external_message_id IS NOT NULL;

CREATE INDEX idx_message_attempts_message_id
    ON message_attempts(message_id);
