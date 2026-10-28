-- A night that produced nothing had no way to say why.
--
-- `LatestUnseen` filtered on `drafted > 0`, so a failed run was invisible and
-- the morning card kept showing the last SUCCESSFUL night as if it were
-- today's. Production ran nine consecutive nights failing every lead for the
-- same reason (no catalogue) while the screen reported "40 abordagens prontas"
-- from nineteen days earlier.
--
-- NULL means the run finished normally — including a legitimately empty night
-- with nobody left to write to, which is not a failure and must not be
-- reported as one.
ALTER TABLE overnight_draft_runs
    ADD COLUMN IF NOT EXISTS stopped_reason TEXT;
