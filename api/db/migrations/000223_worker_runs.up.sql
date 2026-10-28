-- worker_runs: when did each background worker last complete a pass.
--
-- The 24h workers (relationship dates, contract alerts) and the 6h ones
-- (reorder, signal poll) have no boot pass on purpose — a deploy loop must
-- not become three rounds of messages. But with a deploy most days a 24h
-- ticker never fires: the container is replaced first. This stamp lets
-- Start() run on boot ONLY when the last pass is older than the interval,
-- and lets the signal poller skip its auto-offer step on a quick restart.
CREATE TABLE IF NOT EXISTS worker_runs (
    name        TEXT PRIMARY KEY,
    last_run_at TIMESTAMPTZ NOT NULL
);
