-- US-073: capture the request origin (client IP + User-Agent) on every
-- audit row so an admin investigating "who did what" can also see "from
-- where". Both nullable — background/worker writes (cron sweeps, Stripe
-- webhook handlers) have no HTTP request and leave them NULL.
ALTER TABLE audit_logs
    ADD COLUMN ip         TEXT,
    ADD COLUMN user_agent TEXT;
