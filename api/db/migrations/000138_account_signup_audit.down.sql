ALTER TABLE accounts
    DROP COLUMN IF EXISTS signup_ip,
    DROP COLUMN IF EXISTS signup_country,
    DROP COLUMN IF EXISTS signup_locale;
