-- Outbound Sales + CRM (US-022): prospecting searches. Each row records one
-- run of a Google Places prospecting query against a funnel — the criteria
-- (free-text query and/or a lat/long radius), how many candidates Google
-- returned, and how many of them became NEW leads (re-running the same search
-- is idempotent, so leads_created drops to 0 on repeats).
--
-- account_id is denormalised from the funnel (FK CASCADE) so searches can be
-- listed/scoped without a join, mirroring every other account-scoped table.
CREATE TABLE IF NOT EXISTS prospecting_searches (
    id             TEXT PRIMARY KEY,
    account_id     TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    funnel_id      TEXT NOT NULL REFERENCES sales_funnels(id) ON DELETE CASCADE,
    query          TEXT NOT NULL DEFAULT '',
    latitude       DOUBLE PRECISION,
    longitude      DOUBLE PRECISION,
    radius_meters  DOUBLE PRECISION,
    results_count  INTEGER NOT NULL DEFAULT 0,
    leads_created  INTEGER NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_prospecting_searches_account ON prospecting_searches(account_id);
CREATE INDEX IF NOT EXISTS idx_prospecting_searches_funnel ON prospecting_searches(funnel_id);
