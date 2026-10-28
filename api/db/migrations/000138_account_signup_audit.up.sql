-- US-015 (review fixes): record the signup network origin so locale-based
-- currency arbitrage is auditable. Locale stays the currency signal; the
-- IP/country snapshot lets the owner spot a pt-BR user who forced an "en"
-- locale to buy the cheaper USD ladder (or vice versa) and correct the
-- account currency via the owner panel.
ALTER TABLE accounts
    ADD COLUMN signup_ip TEXT,
    ADD COLUMN signup_country TEXT,
    ADD COLUMN signup_locale TEXT;
