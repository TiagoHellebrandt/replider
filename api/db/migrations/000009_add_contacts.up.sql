-- Contacts (CRM-like customer profiles)
CREATE TABLE IF NOT EXISTS contacts (
    id             TEXT PRIMARY KEY,
    account_id     TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    external_id    TEXT NOT NULL,          -- channel-specific ID (phone number, telegram ID, etc.)
    channel_type   TEXT,                   -- where first seen (whatsapp, telegram, etc.)
    name           TEXT,
    email          TEXT,
    phone          TEXT,
    avatar_url     TEXT,
    metadata       JSONB DEFAULT '{}',     -- custom fields
    first_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_contacts_account_external ON contacts(account_id, external_id);
CREATE INDEX idx_contacts_account ON contacts(account_id);
CREATE INDEX idx_contacts_name ON contacts(account_id, name);

-- Link conversations to contacts
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS contact_ref_id TEXT REFERENCES contacts(id) ON DELETE SET NULL;
CREATE INDEX idx_conversations_contact ON conversations(contact_ref_id) WHERE contact_ref_id IS NOT NULL;

-- Conversation Notes (internal comments for team collaboration)
CREATE TABLE IF NOT EXISTS conversation_notes (
    id              TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_name       TEXT NOT NULL DEFAULT '',
    content         TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_conversation_notes_conv ON conversation_notes(conversation_id);

-- Full-text search on messages
ALTER TABLE messages ADD COLUMN IF NOT EXISTS search_vector tsvector
    GENERATED ALWAYS AS (to_tsvector('portuguese', coalesce(content, ''))) STORED;
CREATE INDEX idx_messages_search ON messages USING GIN(search_vector);
