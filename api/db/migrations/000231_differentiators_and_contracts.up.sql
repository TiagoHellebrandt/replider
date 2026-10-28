-- Why yours is better, and who they buy from today.
--
-- The approach had the account's ALLOWED claims (what a seller may say) and
-- its objection library (what the buyer will push back with). It had nothing
-- for the sentence in between: why this equipment rather than the one they
-- already own. So every draft argued from the product's existence, which
-- reads as generic because it is.

-- The account's own differentiators. Unlike objections there is NO code seed:
-- a default list here would be the product inventing advantages about
-- somebody's equipment, and an invented advantage in a message a hospital can
-- check is worse than no advantage at all.
--
-- product_id is optional: most differentiators are the COMPANY's (assistência
-- em 24h, peça em estoque no estado), and those apply to everything it sells.
CREATE TABLE IF NOT EXISTS differentiators (
    id         CHAR(26) PRIMARY KEY,
    account_id CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    product_id TEXT REFERENCES products(id) ON DELETE CASCADE,
    label      TEXT NOT NULL,
    detail     TEXT NOT NULL DEFAULT '',
    position   INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_differentiators_account
    ON differentiators (account_id, position);

-- Public procurement contracts, from PNCP. Shared and account-independent,
-- like the CNES tables: the fact that a hospital signed with a supplier is
-- public record, not one customer's data.
--
-- THIS is the only place a competitor's name may come from, together with the
-- buyer's own words in a conversation. CNES publishes equipment COUNTS and
-- never a brand, so any "they use X" derived from the park would be a guess —
-- and a guess about a supplier, in a message to the hospital that signed with
-- them, is the fastest way to lose the meeting.
CREATE TABLE IF NOT EXISTS public_contracts (
    -- PNCP's own control number ("00394460005887-2-007096/2024").
    pncp_id       TEXT        PRIMARY KEY,
    buyer_cnpj    TEXT        NOT NULL,
    buyer_name    TEXT        NOT NULL DEFAULT '',
    supplier_cnpj TEXT        NOT NULL DEFAULT '',
    supplier_name TEXT        NOT NULL DEFAULT '',
    object        TEXT        NOT NULL DEFAULT '',
    value_cents   BIGINT      NOT NULL DEFAULT 0,
    signed_at     DATE,
    -- When the incumbent's contract runs out: a buying window, and the only
    -- date in this table anybody acts on.
    expires_at    DATE,
    uf            CHAR(2)     NOT NULL DEFAULT '',
    city          TEXT        NOT NULL DEFAULT '',
    fetched_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_public_contracts_buyer
    ON public_contracts (buyer_cnpj, signed_at DESC);

-- When each buyer was last asked about, so a lead drafted twice in a week
-- does not re-fetch. NULL expiry on purpose: absence means "never asked".
CREATE TABLE IF NOT EXISTS public_contract_scans (
    buyer_cnpj TEXT        PRIMARY KEY,
    scanned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    found      INTEGER     NOT NULL DEFAULT 0
);
