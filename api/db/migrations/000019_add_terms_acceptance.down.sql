ALTER TABLE users
    DROP COLUMN IF EXISTS terms_version,
    DROP COLUMN IF EXISTS terms_accepted_at;
