-- The brief becomes three answers instead of one paragraph: what was
-- agreed, what is pending, whose move is next. The dashboard renders them
-- as labeled rows; `text` stays as the flowing form (copy/paste, old rows).
ALTER TABLE conversation_briefs
  ADD COLUMN agreed    TEXT NOT NULL DEFAULT '',
  ADD COLUMN pending   TEXT NOT NULL DEFAULT '',
  ADD COLUMN next_step TEXT NOT NULL DEFAULT '';
