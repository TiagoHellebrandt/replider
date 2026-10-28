CREATE TABLE sales_leads (
    id             TEXT PRIMARY KEY,
    company        TEXT NOT NULL,
    email          TEXT NOT NULL,
    company_size   TEXT NOT NULL,
    channels       TEXT NOT NULL,
    monthly_volume TEXT NOT NULL,
    message        TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Cheap lookup for the per-email rate-limit window (ExistsRecent).
CREATE INDEX idx_sales_leads_email_created_at ON sales_leads (email, created_at);
