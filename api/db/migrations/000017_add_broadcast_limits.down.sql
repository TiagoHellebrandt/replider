ALTER TABLE account_usages DROP COLUMN IF EXISTS broadcast_messages_used;
ALTER TABLE plans
    DROP COLUMN IF EXISTS max_recipients_per_campaign,
    DROP COLUMN IF EXISTS max_broadcast_messages_per_month;
