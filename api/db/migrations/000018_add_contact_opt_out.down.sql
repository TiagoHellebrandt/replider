DROP INDEX IF EXISTS idx_contacts_opted_out;
ALTER TABLE contacts DROP COLUMN IF EXISTS opted_out_at;
