-- Review fix US-014: being a payable seller requires an EXPLICIT owner
-- grant. Before this, any user of the platform account could mint a ref
-- code / convert leads and become payable. seller_settings.is_seller is
-- the flag; DEFAULT FALSE so rows created as side-effects (pix key,
-- activity clause) never grant sellerhood by themselves.
ALTER TABLE seller_settings
    ADD COLUMN IF NOT EXISTS is_seller BOOLEAN NOT NULL DEFAULT FALSE;

-- Existing rows were created deliberately (owner set the activity clause
-- or the user was already operating as a seller in dev/staging — the
-- feature has never been live in prod). Grandfather them in so current
-- sellers don't lose panel access on deploy.
UPDATE seller_settings SET is_seller = TRUE;
