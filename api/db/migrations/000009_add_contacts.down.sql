DROP INDEX IF EXISTS idx_messages_search;
ALTER TABLE messages DROP COLUMN IF EXISTS search_vector;

DROP TABLE IF EXISTS conversation_notes;

ALTER TABLE conversations DROP COLUMN IF EXISTS contact_ref_id;

DROP TABLE IF EXISTS contacts;
