-- US-008 (vendedores v3): objective-event bonuses on the commission
-- ledger. No new tables — the ledger's bonus_* reasons already exist in
-- the 000115 CHECK; upgrade events reuse the append-only billing_events
-- table with the new 'plan_upgraded' event_type (TEXT, no CHECK to
-- alter). This migration only seeds the owner-editable bonus amounts:
--   bonus_nps_cents       fixed bonus when a carteira account answers
--                         NPS 9-10 in the cycle (max 1/account/quarter)
--   bonus_retention_cents one-time bonus when a seller-linked account
--                         completes 12 active months
-- Integer BRL cents; 0 disables the bonus.
INSERT INTO platform_settings (key, value) VALUES
    ('bonus_nps_cents', '10000'),
    ('bonus_retention_cents', '10000')
ON CONFLICT (key) DO NOTHING;
