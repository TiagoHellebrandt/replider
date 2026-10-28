-- US-022: cache embeddings keyed by (text_hash, model) to cut repeated
-- provider embedding calls (~10x fewer Gemini embedding requests on
-- recurring user inputs like greetings or FAQs).
--
-- Lookups are by exact (text_hash, model) key, never by similarity, so
-- the vector is stored untyped (no fixed dimension): the same table can
-- hold 768-dim Gemini vectors and 1536-dim OpenAI vectors side by side.
-- Rows expire after 30 days; expiry is enforced lazily in the read
-- query (WHERE created_at > now() - interval '30 days') and old rows are
-- pruned opportunistically on write, so no cron job is required.
CREATE TABLE IF NOT EXISTS embedding_cache (
    text_hash  TEXT        NOT NULL,
    model      TEXT        NOT NULL,
    embedding  vector      NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (text_hash, model)
);

-- Supports the lazy TTL prune (DELETE ... WHERE created_at < cutoff).
CREATE INDEX IF NOT EXISTS idx_embedding_cache_created_at
    ON embedding_cache (created_at);
