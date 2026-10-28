ALTER TABLE conversations
    DROP COLUMN IF EXISTS sentiment_updated_at,
    DROP COLUMN IF EXISTS sentiment_score,
    DROP COLUMN IF EXISTS sentiment;
