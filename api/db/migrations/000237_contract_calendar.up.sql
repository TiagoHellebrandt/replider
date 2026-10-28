-- The two facts PNCP was already sending and nobody stored.
--
-- Every /contratos row carries `numeroControlePncpAta` and `frutoAdesao`. The
-- first says the purchase was made against a price registry — a twelve-month
-- licence to sell, not a closed deal. The second says this buyer RIDES other
-- bodies' registries, which makes them reachable without winning a tender at
-- all. Both were parsed past and dropped.
ALTER TABLE public_contracts
    ADD COLUMN IF NOT EXISTS ata_pncp_id  TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS from_adhesion BOOLEAN NOT NULL DEFAULT FALSE;

-- The calendar's own index.
--
-- The read is "what runs out in my state between two dates", and it is the
-- whole point of this table: a contract ending is a purchase that MUST happen,
-- on a date published three to six months before any tender exists. Without
-- this index that read is a sequential scan over every contract ever fetched.
CREATE INDEX IF NOT EXISTS idx_public_contracts_expiry
    ON public_contracts (uf, expires_at)
    WHERE expires_at IS NOT NULL;

-- Dates the source got wrong must not become rows nobody can explain.
--
-- Measured on the live API 2026-09-11: a published ata carried
-- "vigenciaFim": "7203-07-27". Left alone it sorts to the bottom of every
-- calendar forever and reads as a five-thousand-year contract. The parser
-- rejects these now; this clears the ones already stored.
UPDATE public_contracts
   SET expires_at = NULL
 WHERE expires_at IS NOT NULL
   AND (expires_at < DATE '1990-01-01' OR expires_at > DATE '2100-01-01');
