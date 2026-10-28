-- Electronic signature (§6.11, G7).
--
-- The provider is BYO: the customer's own ZapSign/Autentique account, their cost,
-- their legal validity (MP 2.200-2 is a product in itself and not one we build).
-- We orchestrate: send the document, follow the status, archive the signed copy,
-- and turn a signature into a Sale so it reaches the portfolio and the commission
-- statement.

CREATE TABLE IF NOT EXISTS signature_settings (
    account_id CHAR(26) PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,

    provider TEXT NOT NULL DEFAULT 'zapsign'
        CHECK (provider IN ('zapsign')),

    -- The customer's API token, encrypted at rest with the same AES-256-GCM
    -- primitive as the channel credentials. bytea + SealBytes, per the house
    -- convention for a new binary column.
    api_token_encrypted BYTEA,

    -- The unguessable token in OUR inbound webhook URL.
    --
    -- ZapSign publishes no HMAC and no shared secret for its webhooks — only
    -- configurable custom headers. So the URL itself is the first credential,
    -- exactly like the per-channel inbound webhook tokens, and the payload is
    -- NEVER trusted to move money on its own: see the re-fetch in the use case.
    webhook_token CHAR(26) NOT NULL,

    -- Sandbox keeps a customer's trials out of their real signature quota.
    sandbox BOOLEAN NOT NULL DEFAULT FALSE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The inbound webhook is routed by this token alone, so it must be unique across
-- accounts and indexed for the lookup.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_signature_webhook_token
    ON signature_settings (webhook_token);

CREATE TABLE IF NOT EXISTS signature_requests (
    id         CHAR(26)    PRIMARY KEY,
    account_id CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    -- What is being signed, when it came from something we already hold. Both
    -- nullable: plenty of proposals are signed before they are a contract, and
    -- plenty of contracts are signed with no lead behind them.
    contract_id CHAR(26) REFERENCES contracts(id) ON DELETE SET NULL,
    lead_id     CHAR(26),
    -- Who signs, for the Sale this becomes. Without a contact the signature can
    -- still be tracked; it just cannot become a sale.
    contact_id CHAR(26),

    provider TEXT NOT NULL DEFAULT 'zapsign',
    -- The provider's own document id. Unique per account: two rows pointing at
    -- one remote document would double-register the sale.
    provider_doc_token TEXT NOT NULL DEFAULT '',

    title TEXT NOT NULL,

    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft','sent','viewed','signed','refused','expired','failed')),
    -- Why it failed, in words a person can act on.
    failure_reason TEXT NOT NULL DEFAULT '',

    -- The signers and their individual state, as the provider reports it.
    signers JSONB NOT NULL DEFAULT '[]',

    -- The PDF we sent, and the signed copy we archived. Archiving is the point:
    -- a signed document that lives only in the provider's account is a document
    -- the customer loses when they stop paying that provider.
    object_path        TEXT NOT NULL DEFAULT '',
    signed_object_path TEXT NOT NULL DEFAULT '',

    -- What this signature is worth, so it can become a Sale. Zero means "track
    -- it, do not register a sale" — a signed document with no value is not a
    -- sale, and inventing one would put a number in somebody's commission.
    value_cents BIGINT NOT NULL DEFAULT 0 CHECK (value_cents >= 0),
    currency    VARCHAR(3) NOT NULL DEFAULT 'brl',

    -- The Sale this produced. Its presence is the idempotency guard: a webhook
    -- that arrives twice must not register two sales.
    sale_id CHAR(26),

    sent_at    TIMESTAMPTZ,
    viewed_at  TIMESTAMPTZ,
    signed_at  TIMESTAMPTZ,
    refused_at TIMESTAMPTZ,

    created_by_user_id CHAR(26),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_signature_requests_account
    ON signature_requests (account_id, created_at DESC);

-- The webhook resolves a request by the provider's document token.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_signature_provider_doc
    ON signature_requests (account_id, provider_doc_token)
    WHERE provider_doc_token <> '';

-- The contract card shows its signature state.
CREATE INDEX IF NOT EXISTS idx_signature_requests_contract
    ON signature_requests (contract_id)
    WHERE contract_id IS NOT NULL;
