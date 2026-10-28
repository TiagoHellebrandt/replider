-- The legacy pipeline incremented account_usages.conversations_count on
-- every AI reply, so a 4-turn thread showed up as "4 conversations".
-- Re-derive the counter from the actual set of conversations created
-- during the current billing cycle — now that the live code only bumps
-- the counter at conversation-create time, this lands the column on
-- the real value and keeps it correct going forward.
UPDATE account_usages au
SET conversations_count = sub.c
FROM (
    SELECT
        au.id,
        (
            SELECT COUNT(*) FROM conversations c
            WHERE c.account_id = au.account_id
              AND c.created_at >= au.billing_cycle_start
        )::int AS c
    FROM account_usages au
) sub
WHERE au.id = sub.id;
