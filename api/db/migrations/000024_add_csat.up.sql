-- Post-conversation satisfaction rating. Customers look at this monthly
-- to justify the subscription — without it Replider is a black box and
-- the first excuse to cancel (e.g. a single bad experience) wins.
--
-- One row per (conversation_id) — we treat a conversation as a unit of
-- work; the customer answers once. rating is 1-5 stars; feedback is the
-- free-text comment (nullable).
CREATE TABLE csat_ratings (
    id              CHAR(26) PRIMARY KEY,
    account_id      CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    conversation_id CHAR(26) NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    contact_id      TEXT NOT NULL,               -- channel-specific id (phone, chat_id, etc.)
    rating          SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    feedback        TEXT,
    sent_at         TIMESTAMPTZ,                 -- when we asked (pending if null)
    answered_at     TIMESTAMPTZ,                 -- when customer replied
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (conversation_id)
);

CREATE INDEX idx_csat_ratings_account ON csat_ratings(account_id, created_at DESC);
