-- Remove the USD tiers. accounts.plan_id references plans with
-- ON DELETE SET NULL, so any subscribed account degrades to the trial
-- limits instead of blocking the rollback.
DELETE FROM plans WHERE id IN (
  '01PLAN_SOLO_USD_0000000',
  '01PLAN_CORE_USD_0000000',
  '01PLAN_PRO_USD_00000000'
);
