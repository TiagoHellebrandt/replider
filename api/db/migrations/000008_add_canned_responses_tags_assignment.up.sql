-- Canned Responses (quick reply templates for agents)
CREATE TABLE IF NOT EXISTS canned_responses (
    id          TEXT PRIMARY KEY,
    account_id  TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    shortcut    TEXT NOT NULL,           -- e.g. "/greeting", "/pricing"
    title       TEXT NOT NULL,           -- display name
    content     TEXT NOT NULL,           -- message content
    category    TEXT DEFAULT 'general',  -- grouping
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_canned_responses_account ON canned_responses(account_id);
CREATE UNIQUE INDEX idx_canned_responses_shortcut ON canned_responses(account_id, shortcut);

-- Conversation Tags
CREATE TABLE IF NOT EXISTS tags (
    id          TEXT PRIMARY KEY,
    account_id  TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    color       TEXT NOT NULL DEFAULT '#6366f1', -- hex color
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_tags_name ON tags(account_id, name);

-- Junction table: conversation <-> tags (many-to-many)
CREATE TABLE IF NOT EXISTS conversation_tags (
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    tag_id          TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (conversation_id, tag_id)
);

CREATE INDEX idx_conversation_tags_tag ON conversation_tags(tag_id);

-- Conversation assignment & SLA fields
ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS assigned_to    TEXT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS priority       TEXT DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    ADD COLUMN IF NOT EXISTS first_response_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS resolved_at    TIMESTAMPTZ;

CREATE INDEX idx_conversations_assigned ON conversations(assigned_to) WHERE assigned_to IS NOT NULL;
CREATE INDEX idx_conversations_priority ON conversations(account_id, priority);
CREATE INDEX idx_conversations_account_created ON conversations(account_id, created_at DESC);

-- Read receipts: track when messages were read by attendant
ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS read_at TIMESTAMPTZ;
