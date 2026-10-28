-- US-F004: multi-channel identity foundation for the SDR pivot.
--
-- Today a contact IS its phone: `contacts.id == external_id == E.164`
-- (see ProspectingUseCase.upsertContact / CreateLeadUseCase). That is fine
-- for a WhatsApp-only product and fatal for an email-first outbound one —
-- a prospect discovered by email has no phone to key on.
--
-- This migration only ADDS the two tables the new model needs. Repointing
-- contacts onto an opaque ULID and migrating the existing FKs happens in
-- US-F005/F006, so this step is non-breaking and independently revertable.
--
-- Scope decision (diverges from the PRD's literal "cnpj UNIQUE" /
-- "UNIQUE(kind,value)"): both uniques are per ACCOUNT. Two Replider
-- customers legitimately prospect the same company, and the same phone can
-- be a contact of two different tenants — a global unique would make one
-- tenant's data block another's write. Cross-account company knowledge is a
-- separate, deliberately-shared cache (US-M001), not this table.

CREATE TABLE IF NOT EXISTS companies (
    id          CHAR(26) PRIMARY KEY,
    account_id  CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name        TEXT     NOT NULL,
    -- Digits only, no mask. NULL for a company we know by name/site alone.
    cnpj        TEXT,
    website     TEXT,
    metadata    JSONB    NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_companies_account ON companies(account_id);
-- Partial unique: many companies per account may have no CNPJ.
CREATE UNIQUE INDEX IF NOT EXISTS idx_companies_account_cnpj
    ON companies(account_id, cnpj) WHERE cnpj IS NOT NULL;

-- A contact's addressable identifiers. One contact, N ways to reach them.
CREATE TABLE IF NOT EXISTS contact_identities (
    id          CHAR(26) PRIMARY KEY,
    account_id  CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    -- TEXT, not CHAR(26): contacts.id is still the phone until US-F005.
    contact_id  TEXT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('phone', 'email', 'linkedin', 'instagram', 'telegram')),
    -- Normalized by the writer: E.164 for phone, lowercased for email.
    value       TEXT NOT NULL,
    -- Stamped when we have positive proof the identifier reaches the person
    -- (they replied on it). Deliverability decisions read this.
    verified_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The resolver's index: "which contact owns this email/phone?" is a single
-- lookup, and the same identifier can't be claimed twice inside an account.
CREATE UNIQUE INDEX IF NOT EXISTS idx_contact_identities_account_kind_value
    ON contact_identities(account_id, kind, value);
CREATE INDEX IF NOT EXISTS idx_contact_identities_contact
    ON contact_identities(contact_id);

-- Contacts belong to a company (B2B: we sell to organizations, we talk to
-- people). Nullable — an inbound WhatsApp contact has no company until
-- enrichment finds one. ON DELETE SET NULL: losing the company record must
-- never cascade into losing the person.
ALTER TABLE contacts ADD COLUMN IF NOT EXISTS company_id CHAR(26)
    REFERENCES companies(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_contacts_company ON contacts(company_id)
    WHERE company_id IS NOT NULL;
