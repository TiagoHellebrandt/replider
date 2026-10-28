-- A/B variant attribution lives on `messages.metadata` JSONB:
--   { "experiment_id": "01H...", "variant": "A", "tokens_in": 120, "tokens_out": 80 }
--
-- The results aggregation joins experiment_assignments to messages
-- and filters by variant. A GIN index keeps that filter off a seq-scan
-- once we've stamped enough rows. Partial because only outbound bot
-- messages carry an experiment_id.
CREATE INDEX idx_messages_metadata_experiment
    ON messages USING gin (metadata jsonb_path_ops)
    WHERE direction = 'outbound' AND metadata ? 'experiment_id';
