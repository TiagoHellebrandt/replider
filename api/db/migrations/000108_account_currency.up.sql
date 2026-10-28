-- US-001: billing currency captured at signup, immutable afterwards.
-- Existing accounts stay 'brl' (the only currency sold to date); new
-- signups from an English locale get 'usd'. No update path exposes this
-- column — currency switching for a live account is out of scope.
ALTER TABLE accounts
    ADD COLUMN currency TEXT NOT NULL DEFAULT 'brl'
    CONSTRAINT accounts_currency_check CHECK (currency IN ('brl', 'usd'));
