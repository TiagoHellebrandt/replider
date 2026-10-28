-- The organization a contact is at, kept where the product can show it.
--
-- `companies` / `contacts.company_id` exist (000147) and nothing in the product
-- writes or reads them. The CSV import meanwhile dropped the company column
-- the moment a row also had a person's name: "Clínica Aurora;Marina" became a
-- contact called Marina at nowhere. A nullable text column on the contact is
-- the smallest thing that keeps the fact visible.
ALTER TABLE contacts ADD COLUMN IF NOT EXISTS company_name TEXT;
