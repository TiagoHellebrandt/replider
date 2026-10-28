-- Explicit flag for whether the account owner has completed the onboarding
-- wizard. Previously the app inferred this from "has any agents" which
-- broke on page reload (creating an agent in step 1 made the backend
-- think onboarding was done, redirecting to /home on refresh).
ALTER TABLE accounts ADD COLUMN onboarding_completed BOOLEAN NOT NULL DEFAULT false;

-- Backfill: accounts that already have agents are considered to have
-- completed onboarding.
UPDATE accounts a
SET onboarding_completed = true
WHERE EXISTS (
    SELECT 1 FROM agents ag WHERE ag.account_id = a.id
);
