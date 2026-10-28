-- Provider batch jobs (custo-BYOK): overnight prompts submitted as one
-- 50%-discounted job, collected by a worker before the morning.
CREATE TABLE IF NOT EXISTS ai_batch_jobs (
    id         CHAR(26) PRIMARY KEY,
    account_id CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL,
    provider   TEXT NOT NULL,
    model      TEXT NOT NULL DEFAULT '',
    job_ref    TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'submitted',
    items      JSONB NOT NULL DEFAULT '[]',
    error      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ai_batch_jobs_status ON ai_batch_jobs (status, created_at);
CREATE INDEX IF NOT EXISTS idx_ai_batch_jobs_account ON ai_batch_jobs (account_id, kind, status);
