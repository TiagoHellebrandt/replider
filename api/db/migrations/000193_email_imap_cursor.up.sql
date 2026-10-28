-- Where the inbox poller stopped reading (US-M005 follow-up: IMAP inbound).
--
-- The customer's provider (Locaweb, Titan, Workspace) is a mailbox, not a
-- webhook: nobody POSTs us their replies, so we go and read them. Which means
-- keeping a cursor, and the shape of that cursor is the whole correctness
-- argument.
--
-- It is a UID, NOT the \Seen flag. The salesperson is reading the same mailbox
-- in Outlook or webmail all day, and \Seen is shared mutable state that they
-- touch: filter on "unseen" and the moment they open a reply on their phone, we
-- lose that message forever. A UID is monotonic and belongs to the server, so
-- "everything above last_uid" is a question only we can answer wrongly.
--
-- uid_validity travels with it because a UID means nothing on its own. When a
-- server rebuilds a mailbox it bumps UIDVALIDITY, and the old UIDs may now
-- address different messages entirely. A changed value means the cursor is
-- garbage and the only safe move is to re-anchor rather than to trust it.
CREATE TABLE IF NOT EXISTS email_imap_cursors (
    channel_id  TEXT PRIMARY KEY REFERENCES channels(id) ON DELETE CASCADE,
    account_id  CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    -- The folder read. INBOX for everything so far; the Sent folder gets its
    -- own row when we start reading what the customer already sent.
    mailbox     TEXT NOT NULL DEFAULT 'INBOX',

    -- Server-assigned generation of the UID space. 0 = never anchored.
    uid_validity BIGINT NOT NULL DEFAULT 0,
    -- Highest UID already ingested. 0 with a set uid_validity means the
    -- mailbox was anchored but nothing read yet, which is how a first
    -- connection avoids importing years of history.
    last_uid    BIGINT NOT NULL DEFAULT 0,

    -- Poll bookkeeping. last_error is kept as TEXT rather than a boolean so a
    -- customer support question ("why did nothing arrive?") has an answer
    -- without reading logs.
    last_polled_at TIMESTAMPTZ,
    last_error     TEXT,
    -- Consecutive failures. A single failed poll is a network blip; a run of
    -- them is a wrong password, and the two deserve different treatment.
    failure_count  INT NOT NULL DEFAULT 0,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The poller claims due rows in order; a partial index would exclude the
-- never-polled ones, which are exactly the ones to run first.
CREATE INDEX IF NOT EXISTS idx_email_imap_cursors_due
    ON email_imap_cursors(last_polled_at NULLS FIRST);
