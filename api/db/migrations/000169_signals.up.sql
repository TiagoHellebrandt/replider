-- Buying signals (§2.1).
--
-- A hospital that publishes a tender has said in writing that it intends to
-- buy a specific thing by a specific date. Stronger intent than anything an
-- enrichment vendor sells, and public.
CREATE TABLE IF NOT EXISTS signals (
    id           CHAR(26)    PRIMARY KEY,
    account_id   CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    source       TEXT        NOT NULL,
    external_id  TEXT        NOT NULL,

    organization TEXT        NOT NULL DEFAULT '',
    cnpj         TEXT        NOT NULL DEFAULT '',
    title        TEXT        NOT NULL DEFAULT '',
    description  TEXT        NOT NULL DEFAULT '',
    url          TEXT        NOT NULL DEFAULT '',

    -- Zero means "not disclosed", which is common in public tenders and is
    -- NOT the same as zero reais. The filters treat it as unknown.
    value_cents  BIGINT      NOT NULL DEFAULT 0,
    city         TEXT        NOT NULL DEFAULT '',
    state        TEXT        NOT NULL DEFAULT '',

    published_at TIMESTAMPTZ NOT NULL,
    -- NULL means no published deadline. Such a signal never expires: hiding
    -- a live opportunity is a worse error than showing an old one, whose
    -- date the salesperson can see.
    closes_at    TIMESTAMPTZ,

    -- Set once somebody acts on it, so two people do not chase the same
    -- tender.
    lead_id      CHAR(26),
    -- A human saying "not for us". Kept rather than deleted so the same
    -- notice does not come back tomorrow.
    dismissed_at TIMESTAMPTZ,

    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The dedup key. PNCP republishes a tender as it is amended, and a
-- salesperson who sees the same notice three times stops reading them.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_signals_account_source_external
    ON signals (account_id, source, external_id);

-- The feed: undismissed, unworked, newest first.
CREATE INDEX IF NOT EXISTS idx_signals_feed
    ON signals (account_id, published_at DESC)
    WHERE dismissed_at IS NULL AND lead_id IS NULL;

-- One account's standing interest.
CREATE TABLE IF NOT EXISTS signal_watches (
    id              CHAR(26)    PRIMARY KEY,
    account_id      CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    source          TEXT        NOT NULL,
    keywords        TEXT[]      NOT NULL DEFAULT '{}',
    cnaes           TEXT[]      NOT NULL DEFAULT '{}',
    states          TEXT[]      NOT NULL DEFAULT '{}',
    min_value_cents BIGINT      NOT NULL DEFAULT 0,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The poller fetches once and fans out to every interested account, so it
-- reads active watches across all tenants.
CREATE INDEX IF NOT EXISTS idx_signal_watches_active
    ON signal_watches (source) WHERE is_active;
