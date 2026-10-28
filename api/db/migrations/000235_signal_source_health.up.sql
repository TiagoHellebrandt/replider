-- Whether the radar's sources are answering, as its own fact.
--
-- The screen had no way to say "PNCP is down". It rendered whatever was
-- stored, so an outage looked exactly like a quiet week: on 2026-09-10 the
-- portal refused every request from 06:20 UTC and the Radar went on showing
-- eight-day-old tenders as if they were today's. A seller cannot tell the
-- difference, and the product had already made this mistake three times
-- elsewhere (the briefing, the ranking, the mini-funnel) — a report filtered
-- to "only what is worth showing" erases its own failure and presents the
-- last good state as current.
--
-- So the failure gets a row of its own. One per source, stamped by the poll:
-- last_attempt_at always, last_success_at only when the source actually
-- answered. The screen compares them.
--
-- Not account-scoped: the sources are shared infrastructure and a portal is
-- either up or it is not. Consecutive_failures is what separates "one bad
-- tick" from "down since yesterday" without keeping a history nobody reads.
CREATE TABLE IF NOT EXISTS signal_source_health (
    source           VARCHAR(32) PRIMARY KEY,
    last_attempt_at  TIMESTAMPTZ NOT NULL,
    last_success_at  TIMESTAMPTZ,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    -- The source's own words, for the operator. Never rendered to a customer:
    -- a seller needs to know the feed is stale, not that page 20 of modality
    -- 8 returned 429.
    last_error       TEXT NOT NULL DEFAULT '',
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
