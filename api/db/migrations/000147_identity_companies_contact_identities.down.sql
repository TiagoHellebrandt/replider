-- Reverts US-F004. Only the objects this migration created are dropped, so
-- a rollback loses the (still unused) identity data and nothing else.
DROP INDEX IF EXISTS idx_contacts_company;
ALTER TABLE contacts DROP COLUMN IF EXISTS company_id;

DROP INDEX IF EXISTS idx_contact_identities_contact;
DROP INDEX IF EXISTS idx_contact_identities_account_kind_value;
DROP TABLE IF EXISTS contact_identities;

DROP INDEX IF EXISTS idx_companies_account_cnpj;
DROP INDEX IF EXISTS idx_companies_account;
DROP TABLE IF EXISTS companies;
