-- Stamp the moment Meta acknowledged "sent" and "failed" for outbound
-- messages so the dashboard can render the four-state ✓ ✓✓ ✓✓📖 ❌
-- timeline accurately. Today only delivered_at and read_at are
-- captured — `sent` is dropped, `failed_reason` is recorded but with
-- no timestamp (so the UI can only show "delivery failed" with no
-- time-of-failure), and any campaign rollup that wants "send rate per
-- minute" had to fall back to messages.created_at (the moment WE
-- queued the row, not the moment Meta confirmed acceptance).
--
-- Both columns nullable: existing rows have no historical signal and
-- backfill would be guesswork.
ALTER TABLE messages
    ADD COLUMN sent_at   TIMESTAMPTZ,
    ADD COLUMN failed_at TIMESTAMPTZ;
