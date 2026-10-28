-- US-M001: shared, company-level enrichment cache.
--
-- The product is vertical (health B2B), so different customers prospect the
-- SAME companies. Paying an LLM/data provider once per customer for the same
-- CNPJ is money burned twice, and it is also a slower answer: a cache hit is
-- milliseconds against seconds of grounded search.
--
-- Deliberately NOT account-scoped — that is the point. The row is keyed by
-- CNPJ alone and readable by every account, which is why what goes IN it is
-- restricted (see below).
--
-- LGPD boundary: this table holds COMPANY facts only (website, address, CNAE,
-- size). A decision-maker's name or LinkedIn is personal data belonging to
-- the account that discovered it (camada 1, private) and must never land
-- here — sharing it across tenants would be desvio de finalidade. The writer
-- enforces an ALLOWLIST of company-level keys rather than a denylist of
-- personal ones, because a denylist silently leaks every field nobody
-- thought of.
CREATE TABLE IF NOT EXISTS company_enrichments (
    -- Digits only, no mask (domain.NormalizeCNPJ).
    cnpj       CHAR(14) PRIMARY KEY,
    -- Company-level facts, allowlisted at write time.
    payload    JSONB    NOT NULL DEFAULT '{}',
    -- Where it came from: 'speedio', 'gemini_grounding', … Kept so a bad
    -- provider's rows can be invalidated wholesale.
    source     TEXT     NOT NULL DEFAULT '',
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Staleness sweeps ("what expired?") and the 90-day TTL read on this.
CREATE INDEX IF NOT EXISTS idx_company_enrichments_fetched_at
    ON company_enrichments(fetched_at);
