-- Recovery for the migration-renumber divergence introduced by the PR #33
-- merge (ralph/audit-2026-05 -> main).
--
-- golang-migrate tracks only the current version NUMBER, not which file
-- produced it. On production, versions 69 and 70 were already consumed by
-- main's lineage (000069 overage_charges table + 000070 reply_reminders
-- columns) BEFORE those files were renumbered to 096/097 during conflict
-- resolution. The branch's own 000069_stripe_event_retry and
-- 000070_daily_token_cap therefore got silently skipped on production —
-- their schema changes never ran.
--
-- Re-deliver those changes here under a fresh version so production
-- converges with what the application code expects. Every statement is
-- idempotent, so on any database that already ran the originals (e.g. CI's
-- fresh DB, which applies 000069/000070 normally) this whole migration is a
-- no-op.

-- ── from 000069_stripe_event_retry ──────────────────────────────────────────
ALTER TABLE processed_stripe_events
    ADD COLUMN IF NOT EXISTS claimed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS processed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS payload      JSONB;

-- Rows that predate the claim/process split were fully processed under the
-- old claim==process logic; backfill processed_at so the reconciler doesn't
-- treat them as stuck. Scoped to NULLs so it's safe to re-run.
UPDATE processed_stripe_events
SET processed_at = created_at
WHERE processed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_processed_stripe_events_stuck
    ON processed_stripe_events (claimed_at)
    WHERE processed_at IS NULL;

-- ── from 000070_daily_token_cap ─────────────────────────────────────────────
-- (also guarded in 000087 so the view there can build; repeated here so the
-- column's origin is recorded with the rest of the skipped-70 recovery.)
ALTER TABLE account_usages
    ADD COLUMN IF NOT EXISTS daily_tokens      BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS daily_tokens_date DATE;
