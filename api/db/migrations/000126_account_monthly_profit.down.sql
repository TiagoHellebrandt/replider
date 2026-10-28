DROP TABLE IF EXISTS account_monthly_profit;

DELETE FROM platform_settings WHERE key IN ('profit_tax_bps', 'infra_cost_cents');
