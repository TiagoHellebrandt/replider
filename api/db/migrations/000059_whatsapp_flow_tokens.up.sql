-- Per-conversation correlation between an outbound Flow message and
-- the inbound nfm_reply submission / data-exchange callbacks.
--
-- When SendFlow ships a Flow CTA, we mint a random `flow_token` and
-- stash it in the action payload. Meta echoes that token on every
-- subsequent data-exchange call AND on the final nfm_reply webhook.
-- Without a server-side index, the token is opaque — we can't tie an
-- LLM-driven dynamic flow back to the conversation that triggered it.
--
-- The token is the unique id (Meta only sends THAT back). flow_id +
-- channel_id + conversation_id are denormalized so the data-exchange
-- handler resolves all three in a single lookup. expires_at lets a
-- daily cleanup worker drop tokens older than the WhatsApp 24h
-- window — Meta won't reuse tokens after that.
CREATE TABLE whatsapp_flow_tokens (
    flow_token       TEXT PRIMARY KEY,
    flow_id          TEXT NOT NULL,                                            -- meta flow id (numeric string)
    channel_id       CHAR(26) NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    conversation_id  CHAR(26) REFERENCES conversations(id) ON DELETE SET NULL, -- nullable: campaign sends have no conv yet
    account_id       CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    recipient        TEXT,                                                     -- E.164 phone we sent to (for audit + dedupe)
    expires_at       TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Cleanup worker drains expired rows.
CREATE INDEX idx_whatsapp_flow_tokens_expires ON whatsapp_flow_tokens(expires_at);

-- Conversation history view: list every flow we sent in a conversation.
CREATE INDEX idx_whatsapp_flow_tokens_conversation ON whatsapp_flow_tokens(conversation_id);
