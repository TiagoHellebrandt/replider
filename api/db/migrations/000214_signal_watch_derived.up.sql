-- Whether a watch's keywords were DERIVED from the account's product
-- catalogue (the seeder) or typed by a person. The signals screen claims
-- "licitações que combinam com o que você vende" — that sentence is only
-- true for derived watches, and the UI needs to know which kind it is
-- showing to stay honest.
ALTER TABLE signal_watches
  ADD COLUMN derived_from_catalog BOOLEAN NOT NULL DEFAULT FALSE;
