-- US-015: remember WHICH platform conversation converted an inbound lead
-- into this customer account, so the seller co-pilot can deep-link the
-- suggestion feed straight back to the client's conversation. Stamped by
-- the explicit convert action (US-004); NULL for ?ref= signups (no
-- conversation ever existed) and for conversions made before this column.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS converted_conversation_id CHAR(26) NULL
        REFERENCES conversations(id) ON DELETE SET NULL;
