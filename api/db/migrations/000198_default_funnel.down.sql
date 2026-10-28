-- The backfilled "Clientes" funnels are left in place: leads may have landed
-- in them since, and a rollback that deletes leads is not a rollback.
DROP INDEX IF EXISTS idx_sales_funnels_one_default_per_account;
ALTER TABLE sales_funnels DROP COLUMN IF EXISTS is_default;
