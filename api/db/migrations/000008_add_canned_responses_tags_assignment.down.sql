ALTER TABLE messages DROP COLUMN IF EXISTS read_at;

ALTER TABLE conversations DROP COLUMN IF EXISTS resolved_at;
ALTER TABLE conversations DROP COLUMN IF EXISTS first_response_at;
ALTER TABLE conversations DROP COLUMN IF EXISTS priority;
ALTER TABLE conversations DROP COLUMN IF EXISTS assigned_to;

DROP TABLE IF EXISTS conversation_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS canned_responses;
