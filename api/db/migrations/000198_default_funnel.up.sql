-- Every account gets ONE funnel it never has to create.
--
-- The funnel was the container every lead had to live in, and the only way
-- to get one was a configuration screen (name, persona, product, webhook)
-- that a distributor opening the app for the first time has no reason to
-- understand. Her mental model is "my customers" — so the container becomes
-- implicit: one default funnel per account, named for what it holds, created
-- with the account, and the destination for anything imported or found
-- without a funnel chosen. Extra funnels are still allowed for whoever wants
-- them; they are just no longer the price of admission.
ALTER TABLE sales_funnels
    ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT FALSE;

-- At most one default per account. Partial: the non-default rows are free.
CREATE UNIQUE INDEX IF NOT EXISTS idx_sales_funnels_one_default_per_account
    ON sales_funnels(account_id) WHERE is_default;

-- Accounts that already have funnels: the oldest active one (or the oldest
-- at all) becomes the default. It is where their leads already are.
WITH pick AS (
    SELECT DISTINCT ON (account_id) id
    FROM sales_funnels
    ORDER BY account_id, active DESC, created_at ASC
)
UPDATE sales_funnels f
SET is_default = TRUE
FROM pick
WHERE f.id = pick.id
  AND NOT EXISTS (
      SELECT 1 FROM sales_funnels d
      WHERE d.account_id = f.account_id AND d.is_default
  );

-- Accounts with no funnel at all get one now, so no screen ever has to say
-- "you have no funnel yet".
INSERT INTO sales_funnels (
    id, account_id, name, persona_prompt, metadata_schema,
    webhook_url, webhook_secret,
    trigger_on_required_complete, trigger_on_buy_intent, active, is_default,
    created_at, updated_at
)
SELECT gen_ulid(), a.id, 'Clientes', '', '[]'::jsonb,
       '', '', FALSE, FALSE, TRUE, TRUE, NOW(), NOW()
FROM accounts a
WHERE NOT EXISTS (SELECT 1 FROM sales_funnels f WHERE f.account_id = a.id);
