-- Who the customer already talks to, learned from her own Sent folder.
--
-- The product's second promise is "traz os clientes e produtos que você já
-- tem", and until now there was no path for it at all: no route creates a
-- contact, products are typed one at a time, and the only import reads Google
-- Places results. Meanwhile the answer was sitting in her mailbox — the people
-- she has been emailing for years ARE her customer list.
--
-- These rows are CANDIDATES, not contacts. Nothing here reaches a customer or
-- becomes a lead until a human picks it: the same posture as the approval queue,
-- for the same reason. A list assembled by a machine from someone's mailbox is
-- exactly the kind of thing that must not act on its own.
--
-- Only headers are read to build this (To/Cc/Date), never message bodies.
-- Reading the full text of every email she ever sent, to answer "who are your
-- customers", would be wildly out of proportion to the question.
CREATE TABLE IF NOT EXISTS discovered_contacts (
    id          CHAR(26) PRIMARY KEY,
    account_id  CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    -- The channel this was learned from, so a later source (a CSV, another
    -- mailbox) can be told apart when the customer asks where a name came from.
    channel_id  TEXT REFERENCES channels(id) ON DELETE SET NULL,

    -- Lowercased bare address. The identity.
    address     TEXT NOT NULL,
    -- Display name from the header, when the sender wrote one. Often absent,
    -- which is fine: the address is what identifies, the name is what a human
    -- recognises.
    name        TEXT,

    -- How many times she wrote to them, and when. This is the whole ranking
    -- signal: somebody emailed 40 times over two years is a customer, somebody
    -- emailed once is a stranger who asked a question.
    sent_count   INT NOT NULL DEFAULT 1,
    first_sent_at TIMESTAMPTZ,
    last_sent_at  TIMESTAMPTZ,

    -- imported_at and dismissed_at are the human's decision, and they are
    -- separate on purpose: "not now" must not look like "already done", or the
    -- next scan re-offers what she already refused.
    imported_at  TIMESTAMPTZ,
    dismissed_at TIMESTAMPTZ,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One row per address per account: a rescan accumulates onto the same
-- candidate instead of duplicating it, and the count keeps meaning something.
CREATE UNIQUE INDEX IF NOT EXISTS idx_discovered_contacts_account_address
    ON discovered_contacts(account_id, address);

-- The review screen reads "most-emailed first, still undecided".
CREATE INDEX IF NOT EXISTS idx_discovered_contacts_pending
    ON discovered_contacts(account_id, sent_count DESC)
    WHERE imported_at IS NULL AND dismissed_at IS NULL;
