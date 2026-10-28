-- Human-readable thread summary (§6.8.2 / spec-telas linha 254).
--
-- Deliberately NOT the existing conversations.summary, which is the rolling
-- digest of turns that aged out of the model's context window: that one is
-- written FOR the model, gets overwritten as the window slides, and says
-- nothing about where the deal stands. This one is written for the
-- salesperson — "negociando desconto de 8%, aguarda aprovação do diretor" —
-- and is what a colleague reads when a lead changes hands.

CREATE TABLE IF NOT EXISTS conversation_briefs (
    conversation_id CHAR(26)    PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
    account_id      CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    text            TEXT        NOT NULL,
    -- Which model wrote it, so a bad summary is traceable to a routing
    -- change rather than argued about.
    model           TEXT        NOT NULL DEFAULT '',

    -- The newest message this brief accounts for. Regeneration is triggered
    -- by comparing against the conversation's latest message: without it the
    -- brief silently describes a negotiation that has since moved on, which
    -- is worse than having none.
    covers_until    TIMESTAMPTZ NOT NULL,
    generated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_conversation_briefs_account
    ON conversation_briefs (account_id);
