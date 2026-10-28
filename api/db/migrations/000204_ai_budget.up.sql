-- Customer-set monthly AI budget (sales-led BYOK cost model): the account's
-- own hard ceiling on AI spend, in BRL cents. NULL = no ceiling set. The
-- spend gate compares the ai_costs ledger (converted at the platform rate)
-- against it and refuses new paid generations once crossed.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS ai_monthly_budget_cents BIGINT;
