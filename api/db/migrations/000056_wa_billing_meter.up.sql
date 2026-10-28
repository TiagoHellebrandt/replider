-- WhatsApp pricing-category metering. Cloud API charges different
-- rates for Marketing (highest), Utility, Authentication, and Service
-- (often free) conversations. The data is captured today on
-- messages.pricing_category + wa_conversation_id but never rolled up
-- to Stripe — meaning every WhatsApp conversation is billed at the
-- flat plan rate regardless of category, leaving real Marketing-heavy
-- accounts under-billed.
--
-- This table is the rollup ledger: one row per
-- (account, wa_conversation_id, pricing_category) so the Stripe
-- pusher worker can dedupe across daily ticks. metered_at non-null
-- means the conversation has already been pushed; the worker only
-- selects rows where it's NULL.
CREATE TABLE wa_billing_meter (
    id              CHAR(26) PRIMARY KEY,
    account_id      CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    wa_conversation_id TEXT NOT NULL,
    pricing_category TEXT NOT NULL,
    conversation_origin TEXT,                   -- referral_conversion / marketing / utility / service / authentication
    pricing_billable BOOLEAN NOT NULL DEFAULT TRUE,
    -- billing_period anchors which monthly window this conversation
    -- counts toward. Computed at insert time as
    -- date_trunc('month', earliest_message_at) so a conversation
    -- straddling a month boundary stays in the month it started.
    billing_period   DATE NOT NULL,
    metered_at      TIMESTAMPTZ,                -- when pushed to Stripe; NULL = pending
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Dedupe across daily-tick reruns: one row per
-- (account, conversation, category). The category is part of the key
-- because Meta can re-classify a conversation as the agent's actions
-- escalate it (utility → marketing), and we want both bills.
CREATE UNIQUE INDEX uniq_wa_billing_meter
    ON wa_billing_meter(account_id, wa_conversation_id, pricing_category);

-- Drain index for the pusher worker: pending rows oldest-first.
CREATE INDEX idx_wa_billing_meter_pending
    ON wa_billing_meter(billing_period, account_id)
    WHERE metered_at IS NULL;

-- Stripe metered subscription items per WhatsApp pricing category.
-- Plans that support WhatsApp metering populate the relevant subset;
-- categories left NULL are billed at the flat plan rate. Updating a
-- plan's price IDs takes effect on the next worker tick — no schema
-- migration per Meta pricing change.
ALTER TABLE plans
    ADD COLUMN stripe_marketing_item_id     TEXT,
    ADD COLUMN stripe_utility_item_id       TEXT,
    ADD COLUMN stripe_authentication_item_id TEXT,
    ADD COLUMN stripe_service_item_id       TEXT;
