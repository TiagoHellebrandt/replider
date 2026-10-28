-- Reverts US-F008/F009. Owners fall back to admin (the pre-migration role
-- every one of them already had), and leads lose ownership metadata.
UPDATE users SET role = 'admin' WHERE role = 'owner';

DROP INDEX IF EXISTS idx_leads_account_owner;
ALTER TABLE leads DROP CONSTRAINT IF EXISTS leads_visibility_check;
ALTER TABLE leads DROP COLUMN IF EXISTS visibility;
ALTER TABLE leads DROP COLUMN IF EXISTS owner_user_id;
