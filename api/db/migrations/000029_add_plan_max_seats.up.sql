-- max_seats caps how many users a workspace can have (counts active
-- attendants + admins, not deleted/deactivated rows). Without this
-- column, InviteTeamMemberUseCase had no plan-defined ceiling so trial
-- accounts could invite unlimited attendants and bypass the billing
-- funnel the rest of the plan-limit checks enforce.
--
-- Seed values mirror the landing page expectations: Starter = solo
-- operator, Pro = small team, Business = mid-team, Scale = "effectively
-- unlimited" (999 — explicit so queries stay sargable vs NULL).
ALTER TABLE plans
  ADD COLUMN max_seats INTEGER NOT NULL DEFAULT 1;

UPDATE plans SET max_seats = 1   WHERE id = '01PLAN_BRONZE_000000000'; -- Starter
UPDATE plans SET max_seats = 3   WHERE id = '01PLAN_SILVER_000000000'; -- Pro
UPDATE plans SET max_seats = 10  WHERE id = '01PLAN_GOLD_0000000000';  -- Business
UPDATE plans SET max_seats = 999 WHERE id = '01PLAN_DIAMOND_00000000'; -- Scale
