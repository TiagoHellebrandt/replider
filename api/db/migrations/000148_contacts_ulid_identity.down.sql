-- Reverts US-F005/F006 by restoring the phone-as-primary-key model from
-- legacy_phone_id. Only contacts that HAVE a legacy id can be restored —
-- rows created after the up migration (born with a ULID and possibly no
-- phone at all) keep their ULID, which is the only non-destructive answer:
-- inventing a phone for them, or deleting them, would lose data.

ALTER TABLE conversations DROP CONSTRAINT IF EXISTS conversations_contact_ref_id_fkey;
ALTER TABLE leads DROP CONSTRAINT IF EXISTS leads_contact_id_fkey;
ALTER TABLE contact_identities DROP CONSTRAINT IF EXISTS contact_identities_contact_id_fkey;

UPDATE conversations conv
SET contact_ref_id = c.legacy_phone_id
FROM contacts c
WHERE conv.contact_ref_id = c.id AND c.legacy_phone_id IS NOT NULL;

UPDATE leads l
SET contact_id = c.legacy_phone_id
FROM contacts c
WHERE l.contact_id = c.id AND c.legacy_phone_id IS NOT NULL;

UPDATE contact_identities ci
SET contact_id = c.legacy_phone_id
FROM contacts c
WHERE ci.contact_id = c.id AND c.legacy_phone_id IS NOT NULL;

ALTER TABLE contacts DROP CONSTRAINT contacts_pkey;
UPDATE contacts SET id = legacy_phone_id WHERE legacy_phone_id IS NOT NULL;
ALTER TABLE contacts ADD PRIMARY KEY (id);

ALTER TABLE conversations
    ADD CONSTRAINT conversations_contact_ref_id_fkey
    FOREIGN KEY (contact_ref_id) REFERENCES contacts(id) ON DELETE SET NULL;
ALTER TABLE leads
    ADD CONSTRAINT leads_contact_id_fkey
    FOREIGN KEY (contact_id) REFERENCES contacts(id) ON DELETE CASCADE;
ALTER TABLE contact_identities
    ADD CONSTRAINT contact_identities_contact_id_fkey
    FOREIGN KEY (contact_id) REFERENCES contacts(id) ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_contacts_legacy_phone;
ALTER TABLE contacts DROP COLUMN IF EXISTS legacy_phone_id;
DROP FUNCTION IF EXISTS gen_ulid();
