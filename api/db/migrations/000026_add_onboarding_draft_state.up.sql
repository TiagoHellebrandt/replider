-- Persist in-progress onboarding state so a user who closes the browser
-- mid-flow lands back on the right step with their drafts preserved on
-- next login. The existing onboarding_* columns (migration 025) only
-- track milestone completion; they can't restore an unsubmitted agent
-- name or a half-typed knowledge blob.
--
--  * onboarding_current_step:
--      Integer in [0, 3] matching the four onboarding screens. Defaults
--      to 0 so new accounts land on the "Create agent" step. Not
--      nullable — every account has an implicit "where am I" even on
--      first login.
--
--  * onboarding_draft:
--      Opaque JSONB bag of free-form draft values (agent name, system
--      prompt, template id, knowledge text, channel selection, etc.).
--      Stored as a blob — the frontend owns the shape, the backend just
--      round-trips it. Defaulted to '{}' so reads never need a NULL
--      check. JSONB (not JSON) for atomic updates and indexability
--      should we ever need to search it.
ALTER TABLE accounts
  ADD COLUMN onboarding_current_step integer NOT NULL DEFAULT 0,
  ADD COLUMN onboarding_draft jsonb NOT NULL DEFAULT '{}'::jsonb;
