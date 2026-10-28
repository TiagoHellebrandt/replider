-- US-025: per-account daily AI token cap.
--
-- The monthly cap alone lets an account burn its entire allowance (or
-- run up large overage) in a single runaway day — a looping integration
-- or abuse. A rolling daily counter bounds the blast radius. The counter
-- resets lazily: AddUsage compares daily_tokens_date to CURRENT_DATE and
-- starts fresh on a new day, so no cron is needed.

ALTER TABLE account_usages
    ADD COLUMN daily_tokens      BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN daily_tokens_date DATE;
