-- US-018: personal canned responses + per-attendant signature.
--
-- canned_responses gains a visibility scope: 'account' rows behave exactly
-- like today (visible to the whole team, managed by admins), 'personal'
-- rows belong to a single user (owner_user_id) and are invisible to
-- everyone else. Personal rows die with their owner (CASCADE) — they are
-- private drafts, not team assets.
ALTER TABLE canned_responses
    ADD COLUMN IF NOT EXISTS visibility TEXT NOT NULL DEFAULT 'account'
        CHECK (visibility IN ('account', 'personal')),
    ADD COLUMN IF NOT EXISTS owner_user_id CHAR(26) REFERENCES users(id) ON DELETE CASCADE;

-- Shortcut uniqueness splits per scope: account-wide shortcuts stay unique
-- per account; personal shortcuts are unique per owner (two attendants can
-- both have a personal "/mine" without colliding — each only sees theirs).
DROP INDEX IF EXISTS idx_canned_responses_shortcut;
CREATE UNIQUE INDEX IF NOT EXISTS idx_canned_responses_shortcut_account
    ON canned_responses(account_id, shortcut) WHERE visibility = 'account';
CREATE UNIQUE INDEX IF NOT EXISTS idx_canned_responses_shortcut_personal
    ON canned_responses(account_id, owner_user_id, shortcut) WHERE visibility = 'personal';

-- Optional per-attendant signature, offered in the composer when sending a
-- MANUAL reply. Never appended to AI responses (client-side insert only).
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS signature TEXT;
