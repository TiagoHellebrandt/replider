-- name: CreateIngestionJob :exec
INSERT INTO ingestion_jobs (id, account_id, knowledge_source_id, type, input, status, created_at)
VALUES ($1, $2, $3, $4, $5, 'pending', NOW());

-- name: GetIngestionJob :one
SELECT id, account_id, knowledge_source_id, type, input, status,
       error_message, chunks_created, started_at, finished_at, created_at
FROM ingestion_jobs
WHERE id = $1 AND account_id = $2
LIMIT 1;

-- name: ListIngestionJobsBySource :many
-- History of ingestion attempts for a given knowledge source. Newest
-- first so the UI can show "last run: 2min ago, 8 chunks".
SELECT id, account_id, knowledge_source_id, type, input, status,
       error_message, chunks_created, started_at, finished_at, created_at
FROM ingestion_jobs
WHERE knowledge_source_id = $1 AND account_id = $2
ORDER BY created_at DESC
LIMIT $3;

-- name: ClaimNextPendingIngestionJob :one
-- Atomic claim-and-run: flips the oldest pending row to 'running' and
-- returns it. SKIP LOCKED lets multiple worker instances coexist
-- without stepping on each other (ready for when we scale past one
-- API pod).
UPDATE ingestion_jobs
SET status = 'running',
    started_at = NOW()
WHERE id = (
    SELECT id FROM ingestion_jobs
    WHERE status = 'pending'
    ORDER BY created_at ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING id, account_id, knowledge_source_id, type, input, status,
          error_message, chunks_created, started_at, finished_at, created_at;

-- name: CompleteIngestionJob :exec
UPDATE ingestion_jobs
SET status = 'completed',
    chunks_created = $2,
    finished_at = NOW()
WHERE id = $1;

-- name: FailIngestionJob :exec
UPDATE ingestion_jobs
SET status = 'failed',
    error_message = $2,
    finished_at = NOW()
WHERE id = $1;
