-- Per-conversation language, detected from the first inbound message so
-- the CSAT prompt (and, later, other system-generated copy) matches what
-- the customer actually wrote. Nullable because legacy conversations
-- and webhook edge cases (no text body) can't be detected; consumers
-- fall back to pt_BR in that case.
ALTER TABLE conversations ADD COLUMN language TEXT;

-- Account-level kill switch for the CSAT asker. Some customers don't
-- want the automatic 1-5 survey (e.g. B2B flows where the attendant
-- already debriefs manually). Default ON to preserve existing behavior.
ALTER TABLE accounts ADD COLUMN csat_enabled BOOLEAN NOT NULL DEFAULT TRUE;
