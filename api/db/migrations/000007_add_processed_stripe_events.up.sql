CREATE TABLE IF NOT EXISTS processed_stripe_events (
    event_id   TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Auto-cleanup events older than 7 days (Stripe retries within 72h max)
CREATE INDEX idx_processed_stripe_events_created_at ON processed_stripe_events (created_at);
