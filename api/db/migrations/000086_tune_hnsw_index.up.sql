-- US-064: pgvector index type tuned (ivfflat vs hnsw).
--
-- RAG semantic search (`AgentRunner.applyRAG`, top-5 cosine) runs on the hot
-- inbound path, so vector-index *query latency* is the metric that matters.
-- The two pgvector index types trade off very differently:
--
--   ivfflat — clusters vectors into `lists` buckets and probes a subset
--     (`ivfflat.probes`) at query time. Cheap to build and small in RAM, but:
--       * recall collapses unless `lists`/`probes` are retuned as the table
--         grows, and the buckets must be *re-trained* (REINDEX) after large
--         ingests or the centroids drift — operationally fragile for a
--         continuously-growing per-tenant knowledge base.
--       * each new chunk lands in whatever bucket exists; incremental inserts
--         degrade recall over time.
--
--   hnsw — a navigable small-world graph. Slower to build and heavier in RAM,
--     but gives the best latency/recall tradeoff at our scale, needs no
--     training step, and absorbs incremental inserts without recall decay.
--     Query latency is tuned per-session with `hnsw.ef_search` (no rebuild).
--
-- Decision: hnsw wins for our access pattern (low-latency top-k on a
-- continuously-ingested, multi-tenant corpus). The index has been hnsw since
-- 000001; this migration makes the build parameters EXPLICIT so the chosen
-- tuning is encoded in the schema rather than relying on pgvector defaults
-- (which can change across extension versions):
--
--   m               = 16  -- max edges per node; pgvector default, good
--                            recall/latency balance for 768-dim cosine.
--   ef_construction = 64  -- build-time candidate list; pgvector default,
--                            larger = better graph quality at higher build cost.
--
-- Query-time recall is set by `hnsw.ef_search` (default 40) at the session
-- level — see docs/db-perf.md — and needs no schema change to adjust.
--
-- See docs/db-perf.md "US-064 — pgvector index type (ivfflat vs hnsw)".
DROP INDEX IF EXISTS idx_knowledge_chunks_embedding;

CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_embedding
    ON knowledge_chunks USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);
