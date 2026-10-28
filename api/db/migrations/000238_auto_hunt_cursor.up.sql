-- Where last night's hunt stopped reading the territory.
--
-- The sweep reads the CNES park ranked by park size and turns what it has
-- never approached into leads. It read from the top every night, bounded to
-- one page, so once that page was in the funnel it could never create another
-- lead: measured on 2026-09-17, 60 hospitals in the funnel out of 2.818
-- matching establishments in the account's territory, and nine consecutive
-- nights of `found=52 created=0`.
--
-- The cursor is what makes the sweep walk the territory instead of guarding
-- its doorstep. It wraps back to 0 at the end, so a new competência — or an
-- establishment a name collision swallowed — gets another pass rather than
-- being written off forever.
CREATE TABLE IF NOT EXISTS auto_hunt_cursors (
    account_id  CHAR(26) PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    -- Rank within "establishments in this territory holding what we sell",
    -- ordered by park size. Not a stable identifier of any establishment:
    -- the ranking is recomputed each month, which is why the walk wraps.
    next_offset INTEGER NOT NULL DEFAULT 0 CHECK (next_offset >= 0),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
