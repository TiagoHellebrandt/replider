-- =============================================================================
-- Replider: Complete Database Schema
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS vector;

-- ---------------------------------------------------------------------------
-- Plans (SaaS tiers)
-- ---------------------------------------------------------------------------
CREATE TABLE plans (
    id                          CHAR(26) PRIMARY KEY,
    name                        VARCHAR(50) NOT NULL,
    stripe_price_id             VARCHAR(100),
    currency                    VARCHAR(10) NOT NULL DEFAULT 'brl',
    price                       INT NOT NULL DEFAULT 0,
    ai_provider                 VARCHAR(50) NOT NULL DEFAULT 'google',
    ai_model                    VARCHAR(100) NOT NULL DEFAULT 'gemini-3.1-flash-live-preview',
    input_multiplier            NUMERIC(10, 4) NOT NULL DEFAULT 1.0,
    output_multiplier           NUMERIC(10, 4) NOT NULL DEFAULT 1.0,
    max_agents                  INT NOT NULL DEFAULT 1,
    max_channels                INT NOT NULL DEFAULT 2,
    max_conversations_per_month INT NOT NULL DEFAULT 100,
    max_ai_tokens_per_month     BIGINT NOT NULL DEFAULT 50000,
    max_storage_mb              INT NOT NULL DEFAULT 100,
    created_at                  TIMESTAMPTZ DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- Accounts
-- ---------------------------------------------------------------------------
CREATE TABLE accounts (
    id                      CHAR(26) PRIMARY KEY,
    name                    VARCHAR(255) NOT NULL,
    plan_id                 CHAR(26) REFERENCES plans(id) ON DELETE SET NULL,
    stripe_customer_id      VARCHAR(100) UNIQUE,
    stripe_subscription_id  VARCHAR(100) UNIQUE,
    subscription_status     VARCHAR(50) DEFAULT 'trialing',
    created_at              TIMESTAMPTZ DEFAULT NOW(),
    updated_at              TIMESTAMPTZ DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- Account Usage (billing cycle tracking)
-- ---------------------------------------------------------------------------
CREATE TABLE account_usages (
    id                      CHAR(26) PRIMARY KEY,
    account_id              CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    billing_cycle_start     DATE NOT NULL,
    conversations_count     INT DEFAULT 0,
    ai_tokens_used          BIGINT DEFAULT 0,
    ai_input_tokens_used    BIGINT DEFAULT 0,
    ai_output_tokens_used   BIGINT DEFAULT 0,
    addon_tokens            BIGINT DEFAULT 0,
    storage_used_mb         INT DEFAULT 0,
    updated_at              TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (account_id, billing_cycle_start)
);

-- ---------------------------------------------------------------------------
-- Users
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id                              CHAR(26) PRIMARY KEY,
    account_id                      CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name                            VARCHAR(255) NOT NULL,
    email                           VARCHAR(255) UNIQUE NOT NULL,
    password_hash                   VARCHAR(255),
    role                            VARCHAR(50) DEFAULT 'attendant',
    is_active                       BOOLEAN DEFAULT true,
    google_id                       VARCHAR(255) UNIQUE,
    meta_id                         VARCHAR(255) UNIQUE,
    reset_token                     VARCHAR(255),
    reset_token_expires_at          TIMESTAMPTZ,
    is_email_verified               BOOLEAN NOT NULL DEFAULT false,
    verification_token              VARCHAR(255),
    verification_token_expires_at   TIMESTAMPTZ,
    created_at                      TIMESTAMPTZ DEFAULT NOW(),
    updated_at                      TIMESTAMPTZ DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- AI Agents
-- ---------------------------------------------------------------------------
CREATE TABLE agents (
    id              CHAR(26) PRIMARY KEY,
    account_id      CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    system_prompt   TEXT,
    is_active       BOOLEAN DEFAULT true,
    provider        VARCHAR(50) NOT NULL DEFAULT 'google',
    model           VARCHAR(100) NOT NULL DEFAULT 'gemini-3.1-flash-live-preview',
    timezone        VARCHAR(100) NOT NULL DEFAULT 'America/Sao_Paulo',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- Messaging Channels (WhatsApp, Telegram)
-- ---------------------------------------------------------------------------
CREATE TABLE channels (
    id              CHAR(26) PRIMARY KEY,
    agent_id        CHAR(26) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    provider        VARCHAR(50) NOT NULL,
    name            VARCHAR(255) NOT NULL DEFAULT '',
    credentials     JSONB NOT NULL,
    webhook_token   VARCHAR(64) UNIQUE,
    is_active       BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- Conversations
-- ---------------------------------------------------------------------------
CREATE TABLE conversations (
    id                          CHAR(26) PRIMARY KEY,
    account_id                  CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    channel_id                  CHAR(26) NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    agent_id                    CHAR(26) REFERENCES agents(id) ON DELETE SET NULL,
    contact_id                  VARCHAR(255) NOT NULL,
    contact_name                VARCHAR(255),
    contact_username            VARCHAR(255),
    status                      VARCHAR(20) NOT NULL DEFAULT 'open',
    is_bot_paused               BOOLEAN NOT NULL DEFAULT false,
    last_customer_message_at    TIMESTAMPTZ,
    waiting_since               TIMESTAMPTZ,
    created_at                  TIMESTAMPTZ DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- Messages
-- ---------------------------------------------------------------------------
CREATE TABLE messages (
    id              CHAR(26) PRIMARY KEY,
    channel_id      CHAR(26) NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    conversation_id CHAR(26) REFERENCES conversations(id) ON DELETE CASCADE,
    sender          VARCHAR(255) NOT NULL,
    sender_name     VARCHAR(255),
    type            VARCHAR(50) NOT NULL,
    content         TEXT,
    media_url       TEXT,
    media_type      VARCHAR(50),
    metadata        JSONB,
    direction       VARCHAR(10) NOT NULL DEFAULT 'inbound',
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- Audit Logs
-- ---------------------------------------------------------------------------
CREATE TABLE audit_logs (
    id          CHAR(26) PRIMARY KEY,
    account_id  CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    actor_id    CHAR(26) REFERENCES users(id) ON DELETE SET NULL,
    action      VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id   CHAR(26) NOT NULL,
    payload     JSONB,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- Knowledge Base (RAG)
-- ---------------------------------------------------------------------------
CREATE TABLE knowledge_sources (
    id          VARCHAR(26) PRIMARY KEY,
    agent_id    CHAR(26) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE knowledge_chunks (
    id          VARCHAR(26) PRIMARY KEY,
    source_id   VARCHAR(26) NOT NULL REFERENCES knowledge_sources(id) ON DELETE CASCADE,
    content     TEXT NOT NULL,
    embedding   vector(768),
    provider    VARCHAR(50),
    source_name VARCHAR(255),
    metadata    JSONB,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- Calendars (appointment booking)
-- ---------------------------------------------------------------------------
CREATE TABLE calendars (
    id                      CHAR(26) PRIMARY KEY,
    agent_id                CHAR(26) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    name                    VARCHAR(255) NOT NULL,
    available_days          INTEGER[] NOT NULL DEFAULT '{1,2,3,4,5}',
    time_slots              JSONB NOT NULL DEFAULT '[]',
    slot_duration_minutes   INT NOT NULL DEFAULT 30,
    google_calendar_id      VARCHAR(255),
    is_active               BOOLEAN DEFAULT true,
    created_at              TIMESTAMPTZ DEFAULT NOW(),
    updated_at              TIMESTAMPTZ DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- Indexes
-- ---------------------------------------------------------------------------

-- Users
CREATE INDEX idx_users_account_id ON users(account_id);
CREATE INDEX idx_users_google_id ON users(google_id) WHERE google_id IS NOT NULL;
CREATE INDEX idx_users_meta_id ON users(meta_id) WHERE meta_id IS NOT NULL;
CREATE INDEX idx_users_reset_token ON users(reset_token) WHERE reset_token IS NOT NULL;
CREATE INDEX idx_users_verification_token ON users(verification_token) WHERE verification_token IS NOT NULL;

-- Agents
CREATE INDEX idx_agents_account_id ON agents(account_id);

-- Channels
CREATE INDEX idx_channels_agent_id ON channels(agent_id);
CREATE INDEX idx_channels_webhook_token ON channels(webhook_token) WHERE webhook_token IS NOT NULL;

-- Conversations
CREATE INDEX idx_conversations_account_status ON conversations(account_id, status);
CREATE INDEX idx_conversations_channel_id ON conversations(channel_id);
CREATE INDEX idx_conversations_waiting_since ON conversations(waiting_since) WHERE waiting_since IS NOT NULL;

-- Messages
CREATE INDEX idx_messages_conversation_id ON messages(conversation_id);
CREATE INDEX idx_messages_created_at ON messages(created_at);

-- Audit Logs
CREATE INDEX idx_audit_logs_account_action ON audit_logs(account_id, action);

-- Knowledge
CREATE INDEX idx_knowledge_sources_agent_id ON knowledge_sources(agent_id);
CREATE INDEX idx_knowledge_chunks_source_id ON knowledge_chunks(source_id);
CREATE INDEX idx_knowledge_chunks_embedding ON knowledge_chunks USING hnsw (embedding vector_cosine_ops);

-- Calendars
CREATE INDEX idx_calendars_agent_id ON calendars(agent_id);
