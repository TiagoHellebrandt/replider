ALTER TABLE csat_ratings DROP CONSTRAINT IF EXISTS csat_ratings_rating_check;
ALTER TABLE csat_ratings ADD CONSTRAINT csat_ratings_rating_check CHECK (rating BETWEEN 1 AND 5);
