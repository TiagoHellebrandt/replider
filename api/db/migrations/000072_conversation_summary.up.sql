-- US-024: rolling summary of messages that have aged out of the AI
-- prompt window. Older turns are folded into this column so the model
-- keeps long-conversation context without us replaying the full (and
-- unbounded) history on every turn.
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS summary TEXT;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS summary_updated_at TIMESTAMPTZ;
