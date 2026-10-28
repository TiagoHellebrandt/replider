-- Three facts the establishment record already published and the loader
-- dropped on the floor.
--
--  * admin_sphere — MUNICIPAL / ESTADUAL / FEDERAL / PRIVADA. It decides the
--    whole sales motion: a public unit buys through a published tender, on a
--    deadline, against competitors whose bids are public; a private one buys
--    when it decides to. The product was writing the same approach for both.
--
--  * has_neonatal_centre / has_obstetric_centre — the qualifier this account's
--    catalogue is made of. It already stored the surgical centre and the
--    hospital-care flag and skipped these two. Measured over the ST file for
--    Mato Grosso on 2026-09-18: 68 establishments run a neonatal centre and
--    122 an obstetric one, out of 11.697.
--
--  * disabled_reason — why the record was deactivated. A closed establishment
--    keeps its equipment rows and its bed census, so without this the hunt
--    would keep offering a hospital that is not open. Empty is the normal
--    case and means operating.
--
-- All three arrive in the same GET the loader already makes. No new request.
ALTER TABLE cnes_establishments ADD COLUMN IF NOT EXISTS admin_sphere TEXT;
ALTER TABLE cnes_establishments ADD COLUMN IF NOT EXISTS has_neonatal_centre BOOLEAN;
ALTER TABLE cnes_establishments ADD COLUMN IF NOT EXISTS has_obstetric_centre BOOLEAN;
ALTER TABLE cnes_establishments ADD COLUMN IF NOT EXISTS disabled_reason TEXT;
ALTER TABLE cnes_establishments ADD COLUMN IF NOT EXISTS cep TEXT;

-- Everything already cached predates these columns, and the loader only
-- re-reads a record after three months — so the radar's top rows, the ones a
-- seller opens today, would be the last to gain them. Same reasoning and same
-- concrete date as migration 000240 (never '-infinity': fetched_at is scanned
-- into a Go time.Time).
UPDATE cnes_establishments SET fetched_at = TIMESTAMPTZ '2000-01-01';
