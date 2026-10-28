DROP INDEX IF EXISTS idx_processed_stripe_events_stuck;

ALTER TABLE processed_stripe_events
    DROP COLUMN IF EXISTS claimed_at,
    DROP COLUMN IF EXISTS processed_at,
    DROP COLUMN IF EXISTS payload;
