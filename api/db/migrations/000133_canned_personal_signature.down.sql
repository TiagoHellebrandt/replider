ALTER TABLE users DROP COLUMN IF EXISTS signature;

DROP INDEX IF EXISTS idx_canned_responses_shortcut_personal;
DROP INDEX IF EXISTS idx_canned_responses_shortcut_account;

-- Restore the original account-wide unique index. Personal rows would
-- collide with account rows under it, so drop them first.
DELETE FROM canned_responses WHERE visibility = 'personal';

ALTER TABLE canned_responses
    DROP COLUMN IF EXISTS owner_user_id,
    DROP COLUMN IF EXISTS visibility;

CREATE UNIQUE INDEX IF NOT EXISTS idx_canned_responses_shortcut
    ON canned_responses(account_id, shortcut);
