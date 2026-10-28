CREATE TABLE roi_leads (
    id                 TEXT PRIMARY KEY,
    email              TEXT NOT NULL,
    messages_per_month INTEGER NOT NULL,
    current_cost       DOUBLE PRECISION NOT NULL,
    hourly_rate        DOUBLE PRECISION NOT NULL,
    estimated_savings  DOUBLE PRECISION NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Cheap lookup for the per-email rate-limit window (ExistsRecent).
CREATE INDEX idx_roi_leads_email_created_at ON roi_leads (email, created_at);
