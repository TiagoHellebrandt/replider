-- Broadcast messages have a per-message cost on Meta's side (conversation
-- charge per recipient). Introducing explicit limits protects both our
-- margins and the tenant from runaway bills if a campaign is misconfigured.
--
-- Two dimensions per plan:
--   * max_broadcast_messages_per_month: aggregate cap across all campaigns
--     in a billing cycle. Enforced at campaign CREATION (total recipients
--     pre-flight) and at WORKER dispatch (defensive re-check).
--   * max_recipients_per_campaign: per-campaign cap (a smaller plan cannot
--     fire a single massive send that burns the whole budget at once).
ALTER TABLE plans
    ADD COLUMN max_broadcast_messages_per_month INT NOT NULL DEFAULT 0,
    ADD COLUMN max_recipients_per_campaign      INT NOT NULL DEFAULT 0;

-- Counter reset on billing cycle rollover (same cadence as other usage).
ALTER TABLE account_usages
    ADD COLUMN broadcast_messages_used INT NOT NULL DEFAULT 0;

-- Seed sensible defaults for existing plans. Fine-grained tuning can
-- be done by admin after this lands.
UPDATE plans SET max_broadcast_messages_per_month = 0,    max_recipients_per_campaign = 0    WHERE name = 'Trial';
UPDATE plans SET max_broadcast_messages_per_month = 1000, max_recipients_per_campaign = 500  WHERE name = 'Starter';
UPDATE plans SET max_broadcast_messages_per_month = 10000,max_recipients_per_campaign = 5000 WHERE name = 'Pro';
UPDATE plans SET max_broadcast_messages_per_month = 50000,max_recipients_per_campaign = 50000 WHERE name = 'Enterprise';
