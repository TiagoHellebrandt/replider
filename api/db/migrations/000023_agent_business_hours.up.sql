-- Business hours per agent. A 24/7 bot is the default (NULL/0), but
-- humanization for Brazilian SMBs benefits hugely from a "we reply 8-18h
-- Mon-Fri" posture — instant 3am replies are the #1 way customers spot
-- the bot. Stored as local-time strings plus a weekday bitmask so we can
-- render them in the UI without parsing timestamps.
ALTER TABLE agents
    ADD COLUMN business_hours_start TEXT,      -- "HH:MM" in agent's timezone
    ADD COLUMN business_hours_end   TEXT,
    ADD COLUMN business_hours_days  SMALLINT NOT NULL DEFAULT 0;  -- bitmask Sun..Sat
