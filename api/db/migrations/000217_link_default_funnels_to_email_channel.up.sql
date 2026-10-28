-- Bind every default funnel that has no channel to the account's oldest
-- active email channel (analysis 2026-09-01, break #1: in prod 7 of 7
-- default funnels had channel_id NULL, so every approval answered
-- LEAD_FUNNEL_NO_CHANNEL). Same rule as usecases.ActiveEmailChannel and
-- FunnelChannelLinker: oldest active email channel wins; a funnel already
-- bound to a channel is a decision and is left alone.
UPDATE sales_funnels f
SET channel_id = c.id,
    updated_at = NOW()
FROM (
    SELECT DISTINCT ON (account_id) id, account_id
    FROM channels
    WHERE provider = 'email' AND is_active
    ORDER BY account_id, created_at
) c
WHERE f.account_id = c.account_id
  AND f.is_default
  AND f.channel_id IS NULL;
