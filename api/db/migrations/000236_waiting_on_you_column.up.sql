-- Give the bottleneck a column on every board that already exists.
--
-- `pending_approval` was left off the default board on the theory that it
-- "lasts seconds to minutes". Production disproved it: on 2026-09-10 two
-- accounts held 89 leads in that state, the oldest for forty days, and it was
-- the largest bucket in the whole pipeline. It rendered folded into
-- "Pesquisados", so a team looking at their board could not see that every
-- offer they had written was stuck in one place.
--
-- Inserted at position 350 — between Pesquisados (300) and Abordados (400) —
-- which is exactly the gap StagePositionStep was left for.
--
-- Idempotent per funnel on two counts: the id is DERIVED from the funnel id
-- (so a re-run writes the same row, not a second one), and a board that
-- already has a column for this status is skipped.
INSERT INTO funnel_stages (id, account_id, funnel_id, name, position, maps_to_status, is_terminal)
SELECT
    -- 26 chars to match the ULID column: a leading '0' plus 25 hex digits.
    -- Deterministic on purpose — see above.
    ('0' || upper(substr(md5('waiting-stage:' || f.id), 1, 25)))::char(26),
    f.account_id,
    f.id,
    'Esperando você',
    350,
    'pending_approval',
    FALSE
FROM sales_funnels f
WHERE NOT EXISTS (
    SELECT 1 FROM funnel_stages s
     WHERE s.funnel_id = f.id
       AND s.maps_to_status = 'pending_approval'
)
  AND NOT EXISTS (
    -- The unique index is on (funnel_id, lower(name)); a board that already
    -- has a column by this name under a different status must not collide.
    SELECT 1 FROM funnel_stages s
     WHERE s.funnel_id = f.id AND lower(s.name) = lower('Esperando você')
)
  AND EXISTS (
    -- Only boards that were actually seeded. A funnel with no stages at all
    -- gets its full default set from the seeder, and dropping one lone column
    -- on it would leave a board with a single column.
    SELECT 1 FROM funnel_stages s WHERE s.funnel_id = f.id
);
