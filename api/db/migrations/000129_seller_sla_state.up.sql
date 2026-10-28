-- US-009 (vendedores v3): mechanical SLA ladder for sellers on the
-- platform account. One row per (seller, cycle) with the measured
-- first-human-response metric and the escalation step:
--   breach cycle 1              -> 'warning'  (ledger untouched)
--   breach cycle 2 consecutive  -> 'reduced'  (profit share at 20%,
--                                  reason 'sla_reduced')
--   breach cycle 3+ consecutive -> 'flag_reallocation' (owner decides
--                                  manually — nothing automatic)
-- A cycle without breach resets the counter. total/breached counts are
-- stored so the owner panel and the seller widget (US-012) can show
-- the % without recomputing.
CREATE TABLE IF NOT EXISTS seller_sla_state (
    id                     CHAR(26) PRIMARY KEY,
    seller_user_id         CHAR(26) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cycle                  CHAR(7) NOT NULL CHECK (cycle ~ '^[0-9]{4}-[0-9]{2}$'),
    total_conversations    INT NOT NULL DEFAULT 0 CHECK (total_conversations >= 0),
    breached_conversations INT NOT NULL DEFAULT 0 CHECK (breached_conversations >= 0),
    breach                 BOOLEAN NOT NULL DEFAULT FALSE,
    consecutive_breaches   INT NOT NULL DEFAULT 0 CHECK (consecutive_breaches >= 0),
    action                 TEXT NOT NULL DEFAULT 'none'
        CHECK (action IN ('none', 'warning', 'reduced', 'flag_reallocation')),
    computed_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (seller_user_id, cycle)
);

CREATE INDEX IF NOT EXISTS idx_seller_sla_state_cycle
    ON seller_sla_state (cycle);

-- Owner-editable SLA parameters (business window 8h-18h America/Cuiaba
-- per the PRD, threshold 4 business hours, breach when >20% of the
-- cycle's assigned conversations blow the threshold).
INSERT INTO platform_settings (key, value) VALUES
    ('sla_first_response_minutes', '240'),
    ('sla_breach_pct', '20'),
    ('sla_business_start_hour', '8'),
    ('sla_business_end_hour', '18'),
    ('sla_timezone', 'America/Cuiaba')
ON CONFLICT (key) DO NOTHING;
