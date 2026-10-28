-- Conversational onboarding (§6.20, K5).
--
-- The wizard asks for a company name, an agent prompt, a knowledge base and a
-- channel before anything happens. Somebody evaluating the product has to supply
-- all of it before seeing a single lead, which is why activation is the
-- bottleneck: the value shows up after the work, not during it.
--
-- A conversation inverts that. One question at a time, each answer immediately
-- worth something, and the whole thing resumable — because the realistic user
-- closes the tab halfway and comes back after lunch.
CREATE TABLE IF NOT EXISTS onboarding_chats (
    account_id CHAR(26) PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,

    -- The state machine's position. A fixed ladder, not free-form chat: the
    -- point is to finish, and a model deciding what to ask next is a model that
    -- can wander.
    step TEXT NOT NULL DEFAULT 'company'
         CHECK (step IN ('company', 'product', 'audiences', 'channel', 'done')),

    -- What has been gathered so far, structured. JSONB rather than columns
    -- because this is a DRAFT: half of it is missing at any moment, and a table
    -- of nullable columns would say nothing about which half.
    answers JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- The transcript, so returning after lunch shows the conversation rather
    -- than an empty box asking a question already answered.
    messages JSONB NOT NULL DEFAULT '[]'::jsonb,

    -- When the account first arrived here and when the first lead landed. The
    -- product's activation metric is the difference — time-to-first-lead — and
    -- measuring it needs both ends stamped, not inferred from created_at.
    started_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    first_lead_at     TIMESTAMPTZ,
    completed_at      TIMESTAMPTZ,

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The activation cohort read: who started, who finished, how long it took.
CREATE INDEX IF NOT EXISTS idx_onboarding_chats_activation
    ON onboarding_chats (started_at DESC);
