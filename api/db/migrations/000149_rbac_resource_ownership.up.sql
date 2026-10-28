-- US-F008/F009: per-resource ownership so a salesperson sees THEIR pipeline.
--
-- users.role is a VARCHAR(50) with no CHECK constraint, so the three new
-- roles (owner, gestor, vendedor) need no type change — the enum lives in
-- domain/rbac.go, which is also where satisfaction is decided. What DOES
-- need schema is ownership: authorization for a lead cannot be answered
-- from the role alone, only from "whose lead is this?".

-- The account creator becomes its owner. Until now the first admin was
-- indistinguishable from any admin invited later, so "only the owner may
-- delete the account / change commission rules" had nothing to key on.
-- Exactly one row per account is promoted (the oldest admin), and only when
-- the account has no owner yet — re-running is a no-op.
WITH first_admin AS (
    SELECT DISTINCT ON (account_id) id, account_id
    FROM users
    WHERE role = 'admin'
    ORDER BY account_id, created_at ASC, id ASC
)
UPDATE users u
SET role = 'owner'
FROM first_admin fa
WHERE u.id = fa.id
  AND NOT EXISTS (
      SELECT 1 FROM users o WHERE o.account_id = u.account_id AND o.role = 'owner'
  );

-- Ownership + visibility on leads. Nullable owner: a freshly prospected lead
-- belongs to nobody until it is assigned, and must stay visible to the team.
ALTER TABLE leads ADD COLUMN IF NOT EXISTS owner_user_id CHAR(26)
    REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE leads ADD COLUMN IF NOT EXISTS visibility TEXT NOT NULL DEFAULT 'team';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'leads_visibility_check'
    ) THEN
        ALTER TABLE leads ADD CONSTRAINT leads_visibility_check
            CHECK (visibility IN ('team', 'private'));
    END IF;
END $$;

-- The vendedor's list query is "my leads in this funnel": account + owner is
-- the access path, so it gets the index.
CREATE INDEX IF NOT EXISTS idx_leads_account_owner
    ON leads(account_id, owner_user_id) WHERE owner_user_id IS NOT NULL;
