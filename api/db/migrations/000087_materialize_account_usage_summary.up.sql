-- US-065: Materialize the per-account usage rollup.
--
-- The dashboard usage endpoint (GET /v1/usage) reads the latest billing-cycle
-- row per account. On account_usages that means a filter + sort
-- (ORDER BY billing_cycle_start DESC LIMIT 1) on every request. This
-- materialized view precomputes "one row per account = latest cycle" so the
-- read collapses to a single unique-index lookup, removing the repeated
-- aggregation from the hot path.
--
-- Refreshed every 5 minutes by services.UsageSummaryRefreshWorker via
-- REFRESH MATERIALIZED VIEW CONCURRENTLY (non-blocking; readers keep the old
-- snapshot until the refresh commits). The view feeds only the read-only
-- dashboard surface — live plan-limit enforcement in ProcessMessage still
-- reads account_usages directly so it never sees stale counters.
--
-- Recovery guard (migration-renumber divergence from the PR #33 merge): on
-- production, version 70 was already consumed by main's lineage, so the
-- branch's 000070_daily_token_cap was silently skipped by golang-migrate and
-- daily_tokens/daily_tokens_date may be absent. Ensure they exist before the
-- view references them. No-op on a fresh DB where 000070 already added them.
ALTER TABLE account_usages
    ADD COLUMN IF NOT EXISTS daily_tokens      BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS daily_tokens_date DATE;

CREATE MATERIALIZED VIEW IF NOT EXISTS account_usage_summary AS
SELECT DISTINCT ON (account_id)
    id,
    account_id,
    billing_cycle_start,
    conversations_count,
    ai_tokens_used,
    ai_input_tokens_used,
    ai_output_tokens_used,
    addon_tokens,
    daily_tokens,
    daily_tokens_date,
    storage_used_mb,
    broadcast_messages_used,
    warned_80_percent,
    warned_100_percent,
    updated_at
FROM account_usages
ORDER BY account_id, billing_cycle_start DESC;

-- A UNIQUE index on account_id is mandatory for REFRESH ... CONCURRENTLY and
-- doubles as the lookup index for the endpoint read.
CREATE UNIQUE INDEX IF NOT EXISTS account_usage_summary_account_id_idx
    ON account_usage_summary (account_id);
