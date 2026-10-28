-- US-169: annual billing. Each plan may carry a second Stripe Price id for
-- the yearly (interval=year) recurring charge, alongside the existing
-- monthly stripe_price_id. NULL means the plan has no annual option yet —
-- checkout then falls back to the monthly price, so enabling annual is
-- incremental and never blocks an upgrade. Contract plans (Partner/
-- Enterprise) keep both columns NULL.
ALTER TABLE plans ADD COLUMN IF NOT EXISTS stripe_price_id_annual VARCHAR(100);
