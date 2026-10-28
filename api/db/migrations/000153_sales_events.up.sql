-- Case instrumentation (US-M011).
--
-- An append-only EVENT log, not a counter table. Counters answer only the
-- questions somebody thought of when they were created and drift silently
-- when a write is missed; events can be re-aggregated over any period, are
-- auditable after the fact, and cost one insert at each pipeline point.
--
-- Deliberately denormalised (source/channel on the row): the aggregation is
-- a GROUP BY over a period, and joining leads/channels for every case report
-- would make the report slower than the pipeline that produced it. It also
-- keeps history honest when a channel is later renamed or deleted.
CREATE TABLE IF NOT EXISTS sales_events (
    id           CHAR(26) PRIMARY KEY,
    account_id   CHAR(26) NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    kind         VARCHAR(32) NOT NULL,
    -- The lead this is about. Nullable + ON DELETE SET NULL: a deleted lead
    -- must not erase the fact that an approach was sent in March.
    lead_id      CHAR(26) REFERENCES leads (id) ON DELETE SET NULL,
    -- Where it came from ('speedio', 'google_places', 'inbound'…) for
    -- lead_created, and which channel type ('whatsapp', 'email'…) for the
    -- send/reply events. Free-form on purpose — a new source must not need
    -- a migration to be measured.
    source       VARCHAR(64) NOT NULL DEFAULT '',
    channel_type VARCHAR(32) NOT NULL DEFAULT '',
    -- Seconds between the outbound approach and this reply. Only set on
    -- reply_received, so "average response time" is an average over rows
    -- that actually have one, never over zeros.
    latency_seconds INTEGER,
    occurred_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The only access pattern: one account, one period, grouped by kind.
CREATE INDEX IF NOT EXISTS idx_sales_events_account_time
    ON sales_events (account_id, occurred_at DESC);

-- A pipeline point that fires twice (retry, redelivered webhook) must not
-- double-count. The unique index makes the insert idempotent per lead+kind
-- for the events that can only happen once per lead.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_sales_events_once_per_lead
    ON sales_events (lead_id, kind)
    WHERE lead_id IS NOT NULL AND kind IN ('lead_created', 'meeting_booked');
