-- Async ingestion pipeline: the knowledge source is created immediately
-- (so the UI has an ID to poll against), and the actual fetch + extract
-- + chunk + embed work is deferred to a worker goroutine that drains
-- pending rows from this table.
--
-- IDs follow the rest of the schema (TEXT/ULID rather than UUID) so the
-- FK types line up with accounts(id) and knowledge_sources(id).
-- Status is a plain TEXT + CHECK constraint to match the campaigns
-- table pattern (000016) — PostgreSQL enums are rigid to alter.

CREATE TABLE ingestion_jobs (
    id                  TEXT PRIMARY KEY,
    account_id          TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    knowledge_source_id TEXT NOT NULL REFERENCES knowledge_sources(id) ON DELETE CASCADE,
    type                TEXT NOT NULL,                -- 'url' | 'pdf' (pdf is a planned next phase)
    input               TEXT NOT NULL,                -- URL for type=url, GCS object_path for type=pdf
    status              TEXT NOT NULL DEFAULT 'pending',
    error_message       TEXT NULL,
    chunks_created      INT  NOT NULL DEFAULT 0,
    started_at          TIMESTAMPTZ NULL,
    finished_at         TIMESTAMPTZ NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (type IN ('url','pdf')),
    CHECK (status IN ('pending','running','completed','failed'))
);

-- Fast path for the worker's "next pending job" query and for the
-- per-account jobs listing on the UI.
CREATE INDEX idx_ingestion_jobs_account_status
    ON ingestion_jobs(account_id, status);

-- Worker-side: cheap "pending, oldest first" scan without a sort.
CREATE INDEX idx_ingestion_jobs_status_created
    ON ingestion_jobs(status, created_at) WHERE status = 'pending';
