-- Price-registry balance and the economic-index cache (§6.11, G5/G6).
--
-- Two additions that turn a reminder into a number somebody can act on:
--
-- 1. The ata de registro de preços. A public body committed to buy, the
--    supplier delivered part of it, and the remaining balance expires with the
--    ata. That balance is pre-approved revenue nobody goes back for.
-- 2. The index the contract is adjusted by. "Reajuste IPCA em março" is a
--    reminder; "you may raise 4.8%, the accumulated IPCA since the base date"
--    is a renegotiation.

ALTER TABLE contracts
    -- A price registry is a different animal from a supply contract: the
    -- balance, not the end date, is what expires first.
    ADD COLUMN IF NOT EXISTS price_registry BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS registry_number TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS registry_balance_cents BIGINT NOT NULL DEFAULT 0
        CHECK (registry_balance_cents >= 0),
    -- The DATE the balance was last known. A balance with no date is a number
    -- nobody can trust — the whole point is telling "R$80k left as of last
    -- week" from "R$80k left as of a year ago".
    ADD COLUMN IF NOT EXISTS registry_balance_on DATE,

    -- When the price was last actually raised. Without it the accumulation
    -- would run from the contract's start forever and double-count every
    -- adjustment already applied.
    ADD COLUMN IF NOT EXISTS last_adjusted_on DATE;

-- The monthly series of each published index, cached.
--
-- No account_id: this is public data and identical for everybody, so it is a
-- SHARED cache like company_enrichments. Two consequences: the e2e suite has
-- to truncate it explicitly (truncating accounts does not reach it), and a
-- past cycle's accumulation never moves because the rows are append-only.
CREATE TABLE IF NOT EXISTS economic_index_values (
    -- ipca | igpm | inpc | igpdi — the four a Brazilian contract actually
    -- names. Stored normalized so "IGP-M", "igpm" and "IGP-M (FGV)" are one row.
    index_code TEXT NOT NULL
        CHECK (index_code IN ('ipca','igpm','inpc','igpdi')),
    -- Always the FIRST day of the reference month: the series is monthly and a
    -- day component would silently split one month into two rows.
    reference_month DATE NOT NULL,
    -- The month's variation in PERCENT (0.62 = 0.62%). NUMERIC, not float:
    -- these are compounded over dozens of months and float drift here becomes
    -- a number somebody disputes.
    monthly_rate NUMERIC(9,6) NOT NULL,

    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (index_code, reference_month)
);

-- The accumulation reads a contiguous range of months for one index.
CREATE INDEX IF NOT EXISTS idx_economic_index_range
    ON economic_index_values (index_code, reference_month);
