-- Migrate the Gemini flash family to gemini-3.7-flash (released 2026-08).
--
-- Scope: every google/gemini plan and agent still on an OLDER flash-tier
-- model moves to gemini-3.7-flash. Pro-tier models and OpenAI configs are
-- deliberately untouched — a customer who picked a pro model made a choice.
-- The set includes gemini-3.1-flash-live-preview, which was never a text
-- model at all (it is the Live/audio API model — the seed bug documented in
-- migration 000125's price list).
--
-- Pricing note (provider_costs below): gemini-3.7-flash launches at an
-- INTRODUCTORY price ($0.75/$3.75 per 1M) that DOUBLES on 2027-01-01
-- ($1.50/$7.50). Both rows are seeded with their effective_from dates so
-- cost attribution flips automatically at the turn of the year.

UPDATE plans
SET ai_model = 'gemini-3.7-flash'
WHERE ai_provider IN ('google', 'gemini')
  AND ai_model IN (
    'gemini-3.1-flash-lite',
    'gemini-3.1-flash-live-preview',
    'gemini-2.5-flash',
    'gemini-2.0-flash',
    'gemini-1.5-flash'
  );

UPDATE agents
SET model = 'gemini-3.7-flash'
WHERE provider IN ('google', 'gemini')
  AND model IN (
    'gemini-3.1-flash-lite',
    'gemini-3.1-flash-live-preview',
    'gemini-2.5-flash',
    'gemini-2.0-flash',
    'gemini-1.5-flash'
  );

-- New agents default to the current flash model, not the audio model.
ALTER TABLE agents ALTER COLUMN model SET DEFAULT 'gemini-3.7-flash';

-- Price list (cents USD per 1M tokens). Fixed ULIDs, idempotent.
INSERT INTO provider_costs (id, provider, model, input_cost_per_1m_cents_usd, output_cost_per_1m_cents_usd, effective_from) VALUES
    ('01K0000000000000000PC0ST05', 'google', 'gemini-3.7-flash',  75, 375, '2026-08-01'),
    ('01K0000000000000000PC0ST06', 'google', 'gemini-3.7-flash', 150, 750, '2027-01-01')
ON CONFLICT (provider, model, effective_from) DO NOTHING;
