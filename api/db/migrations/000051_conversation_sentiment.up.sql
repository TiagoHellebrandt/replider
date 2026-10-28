-- Per-conversation sentiment classification.
--
-- The SentimentAnalyzer service already classifies arbitrary text;
-- Sprint 10 wires it to every inbound message and persists the most
-- recent classification on the conversation row. Tile-level (not
-- per-message) is the right granularity for the dashboard: agents
-- skim "who looks angry?" not "what was the sentiment of message
-- #47?". Score keeps the model's confidence so we can hide noisy
-- "neutral 0.4" classifications later without recomputing.
--
-- All columns nullable so existing rows survive without backfill,
-- and the analyzer can be rate-limited / disabled without breaking
-- the conversations list query.
ALTER TABLE conversations
    ADD COLUMN sentiment           TEXT,        -- positive / neutral / negative / angry
    ADD COLUMN sentiment_score     REAL,        -- 0.0..1.0
    ADD COLUMN sentiment_updated_at TIMESTAMPTZ; -- last analyzer run, used to dedupe rapid inbound bursts
