-- What the company does and where — the two facts the whole offer engine
-- keys on, and the two things onboarding now asks first.
--
-- business_lines: how the company earns — 'sell' (resale of equipment),
-- 'rent' (locação), 'service' (assistência técnica). A hospital equipment
-- distributor is often all three, and each line reads a different signal
-- (a tender is a sale, a new ward is a rental, an idle machine is a
-- service call), so this is a set, not an enum column.
--
-- territory: where she sells, as {"uf": "MT", "cities": ["Cuiabá", ...]}.
-- JSONB because the shape will grow (radius, excluded cities) and because
-- the consumer is a search, not a join.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS business_lines TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS territory      JSONB;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_business_lines_known
    CHECK (business_lines <@ ARRAY['sell', 'rent', 'service']::TEXT[]);
