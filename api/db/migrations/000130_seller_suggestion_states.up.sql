-- US-014 (vendedores v3): per-suggestion dismissed/done state for the
-- seller co-pilot feed. Suggestions themselves are computed on demand
-- (never persisted); only the seller's dismissal survives, keyed by
-- cycle so a dismissed suggestion stays hidden for the rest of the
-- month and naturally resurfaces next cycle if the signal persists.
CREATE TABLE IF NOT EXISTS seller_suggestion_states (
    id CHAR(26) PRIMARY KEY,
    seller_user_id CHAR(26) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    suggestion_type TEXT NOT NULL,
    cycle CHAR(7) NOT NULL CHECK (cycle ~ '^[0-9]{4}-[0-9]{2}$'),
    status TEXT NOT NULL CHECK (status IN ('dismissed', 'done')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (seller_user_id, account_id, suggestion_type, cycle)
);
