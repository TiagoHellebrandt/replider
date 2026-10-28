ALTER TABLE account_usages
    DROP COLUMN IF EXISTS daily_tokens,
    DROP COLUMN IF EXISTS daily_tokens_date;
