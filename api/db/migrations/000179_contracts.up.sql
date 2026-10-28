-- Contract repository (§6.11, G4).
--
-- B2B health is governed by contracts that today live in an email folder:
-- nobody remembers the renewal, the penalty or the index adjustment. This is
-- the place they live, with the fields a model proposes and a HUMAN confirms.
CREATE TABLE IF NOT EXISTS contracts (
    id         CHAR(26)    PRIMARY KEY,
    account_id CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    -- Who it is with. contact_id is nullable because a contract can be signed
    -- before anybody logged the counterpart as a contact.
    contact_id CHAR(26),

    title TEXT NOT NULL,

    -- An amendment (aditivo) chains to its parent and carries the next
    -- version number. Deleting the parent takes the chain with it: an
    -- amendment to a contract that no longer exists amends nothing.
    parent_contract_id CHAR(26) REFERENCES contracts(id) ON DELETE CASCADE,
    version            INT      NOT NULL DEFAULT 1 CHECK (version >= 1),

    -- The PDF in the private bucket. Empty when the contract was registered
    -- from pasted text (a co-pilot cannot hand over bytes).
    object_path TEXT NOT NULL DEFAULT '',
    file_name   TEXT NOT NULL DEFAULT '',

    -- extraction_status: pending → extracting → extracted | failed. A row that
    -- failed keeps whatever a human typed; the failure is about the model, not
    -- about the contract.
    extraction_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (extraction_status IN ('pending','extracting','extracted','failed','skipped')),
    extraction_error  TEXT NOT NULL DEFAULT '',

    -- ai_proposal is what the MODEL said, verbatim and immutable. The columns
    -- below are what a human accepted. Keeping both is what makes "a IA
    -- propõe, o humano valida" auditable instead of a slogan: after somebody
    -- edits a date, the original reading is still there to compare.
    ai_proposal JSONB NOT NULL DEFAULT '{}',

    -- confirmed_at is the human's signature on the reading. Until it is set,
    -- every screen must say the numbers are a proposal — an unconfirmed
    -- extraction driving a renewal alert is a machine making a commercial
    -- decision on its own.
    confirmed_at      TIMESTAMPTZ,
    confirmed_by_user_id CHAR(26),

    -- The confirmed fields. Parties and penalties are lists (JSONB), the rest
    -- are scalars because that is how they are read out loud.
    parties JSONB NOT NULL DEFAULT '[]',
    subject TEXT  NOT NULL DEFAULT '',

    value_cents BIGINT NOT NULL DEFAULT 0,
    currency    VARCHAR(3) NOT NULL DEFAULT 'brl',
    payment_terms TEXT NOT NULL DEFAULT '',

    -- Vigência. Dates, not timestamps: a contract runs on calendar days, and a
    -- timezone on "vence em 31/12" only creates off-by-one arguments.
    starts_on DATE,
    ends_on   DATE,

    -- Auto-renewal plus the notice window is the single most expensive pair of
    -- facts in the document: miss the window and the contract renews itself.
    auto_renewal      BOOLEAN NOT NULL DEFAULT FALSE,
    notice_days       INT     NOT NULL DEFAULT 0 CHECK (notice_days >= 0),

    -- Index and base month for the yearly adjustment ("IPCA em março").
    adjustment_index TEXT NOT NULL DEFAULT '',
    adjustment_month INT  CHECK (adjustment_month IS NULL
                                 OR (adjustment_month BETWEEN 1 AND 12)),

    sla_text     TEXT  NOT NULL DEFAULT '',
    penalties    JSONB NOT NULL DEFAULT '[]',
    exclusivity  BOOLEAN NOT NULL DEFAULT FALSE,
    exclusivity_text TEXT NOT NULL DEFAULT '',
    guarantees   TEXT NOT NULL DEFAULT '',
    termination  TEXT NOT NULL DEFAULT '',
    notes        TEXT NOT NULL DEFAULT '',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- A confirmation must name who confirmed. "Somebody validated this" is not
    -- a fact anybody can act on.
    CONSTRAINT contract_confirmed_by
      CHECK ((confirmed_at IS NULL AND confirmed_by_user_id IS NULL)
             OR (confirmed_at IS NOT NULL AND confirmed_by_user_id IS NOT NULL)),
    -- An amendment cannot be its own parent.
    CONSTRAINT contract_not_own_parent CHECK (parent_contract_id <> id)
);

CREATE INDEX IF NOT EXISTS idx_contracts_account
    ON contracts (account_id, created_at DESC);

-- The renewal/expiry sweep (G6) reads by end date; the partial index keeps it
-- off the rows that have no vigência recorded yet.
CREATE INDEX IF NOT EXISTS idx_contracts_ends_on
    ON contracts (account_id, ends_on)
    WHERE ends_on IS NOT NULL;

-- The amendment chain of one contract.
CREATE INDEX IF NOT EXISTS idx_contracts_parent
    ON contracts (parent_contract_id)
    WHERE parent_contract_id IS NOT NULL;

-- One version number per chain: two "aditivo 2" in the same contract is an
-- argument about which one is in force.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_contract_chain_version
    ON contracts (parent_contract_id, version)
    WHERE parent_contract_id IS NOT NULL;
