-- CSAT pending slots are inserted with rating=0 by CreatePendingCSAT to
-- reserve the (conversation_id) row before the customer answers. The
-- original CHECK (rating BETWEEN 1 AND 5) rejected those inserts, so the
-- csat asker worker failed with SQLSTATE 23514 every tick. Relax the
-- constraint to allow 0 as the "pending" sentinel; analytics already
-- filter with `rating > 0`.
ALTER TABLE csat_ratings DROP CONSTRAINT IF EXISTS csat_ratings_rating_check;
ALTER TABLE csat_ratings ADD CONSTRAINT csat_ratings_rating_check CHECK (rating BETWEEN 0 AND 5);
