-- Objections v1 (tese-agente-autonomo §3): the vertical's objection library
-- and the per-lead anticipated objections the matcher call now returns.

-- The account's own library. Empty means "use the built-in health-vertical
-- seed" (domain.DefaultHealthObjections) — the code default keeps a fresh
-- account covered without a per-account seeding step.
CREATE TABLE IF NOT EXISTS objections (
    id         CHAR(26) PRIMARY KEY,
    account_id CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    label      TEXT NOT NULL,
    counter    TEXT NOT NULL DEFAULT '',
    position   INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_objections_account ON objections (account_id, position);

-- What the matcher anticipated for THIS lead: [{label, counter}].
ALTER TABLE leads
    ADD COLUMN IF NOT EXISTS anticipated_objections JSONB NOT NULL DEFAULT '[]';
