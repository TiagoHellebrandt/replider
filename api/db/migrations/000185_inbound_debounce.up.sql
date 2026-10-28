-- Inbound debounce (§6.15, K1).
--
-- A person types the way people type: "oi" / "tudo bem?" / "qual o preço?" in
-- three messages over eight seconds. Today that is three paid messages and three
-- model calls, and — worse for the customer — three separate replies to what was
-- one question.
--
-- So an inbound waits for a moment of silence before the AI answers. The wait is
-- a ROW, not an in-process timer, for two reasons: a deploy in the middle of a
-- burst must not swallow the reply, and two pods must not each answer the same
-- burst.
CREATE TABLE IF NOT EXISTS inbound_debounce (
    -- One pending run per conversation. The collapse IS the feature: a second
    -- message extends the wait instead of queueing a second reply.
    conversation_id CHAR(26) PRIMARY KEY
                    REFERENCES conversations(id) ON DELETE CASCADE,
    account_id      CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    channel_id      CHAR(26) NOT NULL,
    -- The agent that will answer. Stored rather than re-resolved so a channel
    -- repointed mid-wait does not change who replies to a burst already in
    -- flight.
    agent_id        CHAR(26) NOT NULL,

    -- When the AI may answer. Pushed forward by each new inbound, but never
    -- past the ceiling derived from first_queued_at (see the worker).
    respond_at      TIMESTAMPTZ NOT NULL,
    -- The ceiling's anchor. A customer who keeps typing must not defer the
    -- answer forever: at some point "waiting for them to finish" becomes
    -- "ignoring them".
    first_queued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- How many inbounds collapsed into this one run. Logged, and shown to the
    -- attendant, because "aguardando o cliente terminar (3 mensagens)" explains
    -- the delay that would otherwise look like a stuck bot.
    message_count   INTEGER NOT NULL DEFAULT 1,
    -- The LAST message's text: the thing to answer. The history the pipeline
    -- replays comes from the messages table, so the whole burst is seen either
    -- way — this is the trigger, not the transcript.
    content         TEXT NOT NULL DEFAULT '',

    -- Claimed by a worker. Set on claim and cleared by deletion, so a worker
    -- that dies mid-run leaves a row that the stale-claim sweep re-queues
    -- rather than a reply that never happens.
    claimed_at      TIMESTAMPTZ,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The worker's read: what is due and unclaimed, oldest first.
CREATE INDEX IF NOT EXISTS idx_inbound_debounce_due
    ON inbound_debounce (respond_at)
    WHERE claimed_at IS NULL;

-- The stale-claim sweep.
CREATE INDEX IF NOT EXISTS idx_inbound_debounce_claimed
    ON inbound_debounce (claimed_at)
    WHERE claimed_at IS NOT NULL;

-- Per-account configuration.
--
-- NULL means "use the platform default", which is NOT the same as 0: zero is an
-- explicit "answer immediately", the pre-debounce behaviour, and an account that
-- wants it must be able to say so without the column being read as unset.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS inbound_debounce_seconds INTEGER;

COMMENT ON COLUMN accounts.inbound_debounce_seconds IS
    'Silence window before the AI answers an inbound burst. NULL = platform default, 0 = answer immediately.';
