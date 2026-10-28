-- US-011: claim-and-retry for Stripe webhook events.
--
-- The old idempotency table collapsed "claim" and "processed" into a
-- single INSERT, so a handler that crashed AFTER claiming but BEFORE
-- finishing its side-effects left a row that looked done forever — the
-- event was silently dropped (no usage reset, no subscription activation).
--
-- Split the two: claimed_at marks "a worker is on it", processed_at marks
-- "side-effects committed". A background reconciler re-runs any row that
-- has been claimed for >5m without completing. payload stores the raw
-- Stripe event so the reconciler can replay it without a Stripe round-trip.

ALTER TABLE processed_stripe_events
    ADD COLUMN claimed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN processed_at TIMESTAMPTZ,
    ADD COLUMN payload      JSONB;

-- Existing rows were fully processed under the old claim==process logic.
UPDATE processed_stripe_events
SET processed_at = created_at
WHERE processed_at IS NULL;

-- Reconciler scans only stuck claims (claimed but never finished); a
-- partial index keeps it tiny since the common case is processed_at NOT NULL.
CREATE INDEX idx_processed_stripe_events_stuck
    ON processed_stripe_events (claimed_at)
    WHERE processed_at IS NULL;
