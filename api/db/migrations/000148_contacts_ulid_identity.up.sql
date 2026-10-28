-- US-F005/F006: give contacts an opaque id and repoint every reference.
--
-- Until now `contacts.id` WAS the phone number (see the pre-pivot
-- ProspectingUseCase.upsertContact / CreateLeadUseCase, which set
-- id == external_id == E.164). Consequences we are removing here:
--
--   * a prospect known only by work email cannot be stored at all;
--   * a contact who changes phone number becomes a different person,
--     orphaning their lead history;
--   * every FK to contacts leaks PII into other tables' data.
--
-- Three FKs point at contacts (conversations.contact_ref_id,
-- leads.contact_id, contact_identities.contact_id); all three are
-- repointed here in one transaction, so there is no window where a child
-- row references a missing parent.
--
-- `legacy_phone_id` keeps the old value. It is transitional (the down
-- migration restores from it, and any straggler code path can still map
-- old→new), NOT a permanent second identity — the phone lives in
-- contact_identities from now on.

-- gen_ulid mints a Crockford-base32 ULID: 48-bit millisecond timestamp +
-- 80 bits of randomness, so ids sort by creation time like the ones Go
-- generates with oklog/ulid. Needed because the backfill has to mint one
-- id per existing row and golang-migrate only runs SQL.
CREATE OR REPLACE FUNCTION gen_ulid() RETURNS TEXT AS $$
DECLARE
    encoding  TEXT = '0123456789ABCDEFGHJKMNPQRSTVWXYZ';
    unix_ms   BIGINT;
    output    TEXT = '';
    i         INT;
    rand_byte INT;
BEGIN
    unix_ms := (EXTRACT(EPOCH FROM CLOCK_TIMESTAMP()) * 1000)::BIGINT;

    -- 10 characters of timestamp (50 bits available, 48 used — the two
    -- high bits are always zero until the year 10889).
    FOR i IN REVERSE 9..0 LOOP
        output := SUBSTRING(encoding FROM ((unix_ms >> (i * 5)) & 31)::INT + 1 FOR 1) || output;
    END LOOP;
    output := REVERSE(output);

    -- 16 characters of randomness.
    FOR i IN 1..16 LOOP
        rand_byte := FLOOR(RANDOM() * 32)::INT;
        output := output || SUBSTRING(encoding FROM rand_byte + 1 FOR 1);
    END LOOP;

    RETURN output;
END;
$$ LANGUAGE plpgsql;

-- 1. Preserve the old key and mint the new one.
ALTER TABLE contacts ADD COLUMN IF NOT EXISTS legacy_phone_id TEXT;
ALTER TABLE contacts ADD COLUMN IF NOT EXISTS new_id CHAR(26);

UPDATE contacts SET legacy_phone_id = id WHERE legacy_phone_id IS NULL;
UPDATE contacts SET new_id = gen_ulid() WHERE new_id IS NULL;

-- 2. Every existing contact keeps its old identifier — as an identity row
-- now, not as its primary key. The kind depends on where we met them:
-- an E.164 external_id is a phone; a Telegram/Instagram id is that
-- platform's handle. The internal CS channel stores a USER ULID in
-- external_id — that is not a way to reach a person, so it is skipped.
INSERT INTO contact_identities (id, account_id, contact_id, kind, value, created_at, updated_at)
SELECT gen_ulid(), c.account_id, c.id,
       CASE
           WHEN c.external_id ~ '^\+[1-9][0-9]{1,14}$' THEN 'phone'
           WHEN c.channel_type = 'telegram'  THEN 'telegram'
           WHEN c.channel_type = 'instagram' THEN 'instagram'
           ELSE 'phone'
       END,
       c.external_id,
       NOW(), NOW()
FROM contacts c
WHERE c.external_id IS NOT NULL
  AND c.external_id <> ''
  -- Only rows we can actually address; an internal-channel contact keyed
  -- by user ULID has no reachable identifier.
  AND (c.external_id ~ '^\+[1-9][0-9]{1,14}$' OR c.channel_type IN ('telegram', 'instagram'))
ON CONFLICT (account_id, kind, value) DO NOTHING;

-- Email is a second identity when the CRM already knew one.
INSERT INTO contact_identities (id, account_id, contact_id, kind, value, created_at, updated_at)
SELECT gen_ulid(), c.account_id, c.id, 'email', LOWER(TRIM(c.email)), NOW(), NOW()
FROM contacts c
WHERE c.email IS NOT NULL
  AND TRIM(c.email) <> ''
  AND LOWER(TRIM(c.email)) ~ '^[^\s@]+@[^\s@.]+(\.[^\s@.]+)+$'
ON CONFLICT (account_id, kind, value) DO NOTHING;

-- 3. Repoint children BEFORE swapping the key, with the FKs dropped so the
-- intermediate state is legal. Re-added at the end.
ALTER TABLE conversations DROP CONSTRAINT IF EXISTS conversations_contact_ref_id_fkey;
ALTER TABLE leads DROP CONSTRAINT IF EXISTS leads_contact_id_fkey;
ALTER TABLE contact_identities DROP CONSTRAINT IF EXISTS contact_identities_contact_id_fkey;

UPDATE conversations conv
SET contact_ref_id = c.new_id
FROM contacts c
WHERE conv.contact_ref_id = c.id;

UPDATE leads l
SET contact_id = c.new_id
FROM contacts c
WHERE l.contact_id = c.id;

UPDATE contact_identities ci
SET contact_id = c.new_id
FROM contacts c
WHERE ci.contact_id = c.id;

-- 4. Swap the primary key. The unique index on (account_id, external_id)
-- survives untouched — external_id remains the channel-facing identifier.
ALTER TABLE contacts DROP CONSTRAINT contacts_pkey;
UPDATE contacts SET id = new_id;
ALTER TABLE contacts ADD PRIMARY KEY (id);
-- ALLOW-DESTRUCTIVE: new_id is a scratch column created six statements above
-- in this same migration and already copied into id. Dropping it loses
-- nothing; the old key survives in legacy_phone_id.
ALTER TABLE contacts DROP COLUMN new_id;

-- 5. Restore referential integrity with the same delete semantics as before.
ALTER TABLE conversations
    ADD CONSTRAINT conversations_contact_ref_id_fkey
    FOREIGN KEY (contact_ref_id) REFERENCES contacts(id) ON DELETE SET NULL;
ALTER TABLE leads
    ADD CONSTRAINT leads_contact_id_fkey
    FOREIGN KEY (contact_id) REFERENCES contacts(id) ON DELETE CASCADE;
ALTER TABLE contact_identities
    ADD CONSTRAINT contact_identities_contact_id_fkey
    FOREIGN KEY (contact_id) REFERENCES contacts(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_contacts_legacy_phone
    ON contacts(account_id, legacy_phone_id) WHERE legacy_phone_id IS NOT NULL;
