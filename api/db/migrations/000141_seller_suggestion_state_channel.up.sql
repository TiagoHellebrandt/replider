-- Review-fixes US-022: channel-scoped co-pilot dismissals. The
-- whatsapp_ban_risk suggestion is emitted per RED channel, so its
-- dismissal must be keyed by channel too — dismissing one number's risk
-- must not hide a DIFFERENT number that goes RED later in the cycle.
-- Account-level suggestion types keep channel_id = '' (part of the
-- unique key; NULL would break the uniqueness guarantee).
ALTER TABLE seller_suggestion_states
    ADD COLUMN IF NOT EXISTS channel_id TEXT NOT NULL DEFAULT '';

-- The original inline UNIQUE got an auto-truncated name
-- (seller_suggestion_states_seller_user_id_account_id_suggesti_key on a
-- fresh DB). Drop whatever unique constraint exists on the table by
-- catalog lookup — a name literal + IF EXISTS could silently no-op if
-- an environment truncated differently (US-023 concern class).
DO $$
DECLARE
    cname text;
BEGIN
    FOR cname IN
        SELECT conname FROM pg_constraint
        WHERE conrelid = 'seller_suggestion_states'::regclass
          AND contype = 'u'
    LOOP
        EXECUTE format('ALTER TABLE seller_suggestion_states DROP CONSTRAINT %I', cname);
    END LOOP;
END $$;

ALTER TABLE seller_suggestion_states
    ADD CONSTRAINT uniq_seller_suggestion_states
    UNIQUE (seller_user_id, account_id, suggestion_type, channel_id, cycle);
