-- US-021 (review fixes): persist the round-robin cursor. The old
-- in-memory per-process counter reset to the lowest-ULID attendant on
-- every deploy and diverged across replicas. One row per account holds
-- the last assigned user id; the next pick is the smallest pool id
-- greater than it (wrapping to the smallest), so a stale cursor (user
-- removed from the pool) degrades gracefully — no FK to users on
-- purpose.
CREATE TABLE IF NOT EXISTS auto_assign_rotation (
    account_id CHAR(26) PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    last_assigned_user_id TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
