-- Which of the account's products this lead's offer is about, and why.
--
-- Until now the draft saw ONE product — the one configured on the funnel —
-- and the default funnel has none, so an account that imported a catalogue
-- of forty items wrote its first offer knowing nothing about any of them.
-- The matcher picks one or two per lead (by the lead's dossier against the
-- catalogue) and records the pick here so the card, the approval screen and
-- the draft all read the same choice. Shape: [{"product_id","name","reason"}].
ALTER TABLE leads
    ADD COLUMN IF NOT EXISTS matched_products JSONB NOT NULL DEFAULT '[]'::jsonb;
