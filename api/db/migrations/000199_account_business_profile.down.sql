ALTER TABLE accounts DROP CONSTRAINT IF EXISTS accounts_business_lines_known;
ALTER TABLE accounts
    DROP COLUMN IF EXISTS territory,
    DROP COLUMN IF EXISTS business_lines;
