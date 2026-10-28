-- Prompt library + versioning.
--
-- `prompts` is the user-facing "Meus Prompts" catalogue: each row is a
-- named system prompt the user authored (via the wizard, manually, or
-- duplicated from another). The wizard answers are kept verbatim so a
-- future re-synthesis can run with the same intent without re-asking.
--
-- `prompt_versions` is the per-agent audit trail. Every time a prompt
-- (or free-text body) is applied to an agent we snapshot the body that
-- *was active before* into a new row, so rollback is a 1-tap revert
-- even after the source prompt is renamed or deleted.
--
-- FK from `prompt_versions.prompt_id` is `ON DELETE SET NULL` (not
-- CASCADE) on purpose: deleting the library entry must not break the
-- agent's history.
CREATE TABLE prompts (
    id              CHAR(26) PRIMARY KEY,                                       -- ULID
    account_id      CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name            VARCHAR(120) NOT NULL,
    description     TEXT,
    body            TEXT NOT NULL,                                              -- the system prompt itself (post-synthesis)
    source          VARCHAR(20) NOT NULL DEFAULT 'wizard',                      -- wizard | manual | duplicated | template
    wizard_answers  JSONB,                                                      -- raw answers (typed shape lives in domain.WizardAnswers)
    created_by      CHAR(26) REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Listing on the "Meus Prompts" tab is account-scoped, recency-first.
CREATE INDEX idx_prompts_account_updated ON prompts(account_id, updated_at DESC);

-- Per-account name uniqueness (case-insensitive). Stops accidental
-- duplicate names; the UI can suggest "Copy of …" on conflict.
CREATE UNIQUE INDEX uniq_prompts_account_name ON prompts(account_id, lower(name));

CREATE TABLE prompt_versions (
    id          CHAR(26) PRIMARY KEY,                                           -- ULID
    agent_id    CHAR(26) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    prompt_id   CHAR(26) REFERENCES prompts(id) ON DELETE SET NULL,             -- nullable: free-text applies have no library row
    body        TEXT NOT NULL,                                                  -- snapshot of agent.system_prompt BEFORE this apply
    applied_by  CHAR(26) REFERENCES users(id) ON DELETE SET NULL,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    note        TEXT                                                            -- free-form audit comment
);

-- Versions tab list: by agent, newest first.
CREATE INDEX idx_prompt_versions_agent_applied ON prompt_versions(agent_id, applied_at DESC);
