-- Mailbox history as per-contact memory (issue #292, v1 deterministic).
--
-- The discovery scan already accumulates the relationship's shape per address
-- (sent_count, first/last_sent_at). last_subject adds the one envelope field
-- that says what the relationship was ABOUT — still envelope-only, never a
-- body. Written by the scan when a sighting is the newest one; read by the
-- lead dossier so a draft can reference the real last thread instead of
-- opening cold.
ALTER TABLE discovered_contacts
    ADD COLUMN IF NOT EXISTS last_subject TEXT;
