-- Dropping this loses the candidates AND the record of which ones a human
-- already dismissed, so a later scan re-offers names she refused.
DROP TABLE IF EXISTS discovered_contacts;
