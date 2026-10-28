-- Channel-scoped rows cannot survive without the column: keep one row
-- per (seller, account, type, cycle) — the newest — before restoring
-- the narrower unique key.
ALTER TABLE seller_suggestion_states
    DROP CONSTRAINT IF EXISTS uniq_seller_suggestion_states;

DELETE FROM seller_suggestion_states s
USING seller_suggestion_states keep
WHERE s.seller_user_id = keep.seller_user_id
  AND s.account_id = keep.account_id
  AND s.suggestion_type = keep.suggestion_type
  AND s.cycle = keep.cycle
  AND s.id < keep.id;

ALTER TABLE seller_suggestion_states
    DROP COLUMN IF EXISTS channel_id;

ALTER TABLE seller_suggestion_states
    ADD CONSTRAINT uniq_seller_suggestion_states_legacy
    UNIQUE (seller_user_id, account_id, suggestion_type, cycle);
