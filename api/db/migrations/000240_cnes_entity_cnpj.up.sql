-- Who actually signs the contracts for a hospital that has no CNPJ of its own.
--
-- A public hospital is usually not a legal person: it is a unit of a
-- secretaria. DATASUS says so in the same record we already fetch — it
-- publishes `numero_cnpj_entidade` (the maintaining entity) beside
-- `numero_cnpj` (the establishment's own, often null) — and the loader read
-- the second and dropped the first.
--
-- Measured over the 20 biggest parks in Mato Grosso on 2026-09-18: 10
-- establishments publish their own CNPJ, 11 publish the entity's, and NONE
-- publish neither. The field the radar needed for every one of them was
-- already arriving in the payload.
--
-- It is a SEPARATE column, never a fallback into `cnpj`, because the two
-- answer different questions. Seven of those twenty hospitals share
-- 57252971000146 (Secretaria Estadual de Saúde de MT). Folding the entity
-- into `cnpj` would make seven distinct hospitals look like one buyer,
-- attribute the secretaria's contracts to each of them, and print the same
-- competitive claim on seven cards. The entity is who BUYS FOR the hospital;
-- it is not the hospital.
ALTER TABLE cnes_establishments ADD COLUMN IF NOT EXISTS cnpj_entidade TEXT;

-- Make every record that could gain one due for a refresh.
--
-- The worker re-reads an establishment only when its stamp is older than
-- three months, so without this the column would fill over a quarter and the
-- radar's top rows — the ones a seller opens today — would be the last to
-- get it. Restricted to rows with no CNPJ at all, so nothing already answered
-- is re-fetched, and bounded by the loader's own per-tick cap (1500) rather
-- than by this statement.
--
-- A concrete old date, NOT '-infinity': fetched_at is scanned into a Go
-- time.Time, and an infinite timestamp is exactly the kind of value that
-- reads back as a parse failure nobody tests for.
UPDATE cnes_establishments
SET fetched_at = TIMESTAMPTZ '2000-01-01'
WHERE cnpj IS NULL OR trim(cnpj) = '';
