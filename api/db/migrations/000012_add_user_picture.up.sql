-- Adds an optional profile picture URL to users. Populated for users who
-- sign in via Google (from the OAuth `picture` claim). NULL for everyone
-- else, in which case the UI falls back to an initials-based avatar.
ALTER TABLE users ADD COLUMN picture TEXT NULL;
