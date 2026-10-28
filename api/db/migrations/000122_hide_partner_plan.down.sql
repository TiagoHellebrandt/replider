-- Revert US-001: drop the listing gate (Partner becomes visible again
-- implicitly once the column is gone — the handler treats a missing
-- column build as "everything listed").
ALTER TABLE plans DROP COLUMN IF EXISTS is_listed;
