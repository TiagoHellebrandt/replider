-- Cached prompt tokens on the AI cost ledger (§6.16, K3).
--
-- K3 is a claim about ORDER: put everything stable at the front of the prompt and
-- the provider serves it from its prefix cache at a fraction of the input rate. A
-- claim about cost with no number behind it is the kind this codebase refuses
-- everywhere else, so the cached count is recorded per call.
--
-- A SUBSET of input_tokens, never an addition: every provider reports the cached
-- count as part of the prompt total. Summing the two would double-bill the half
-- that was discounted.
ALTER TABLE ai_costs
    ADD COLUMN IF NOT EXISTS cached_tokens BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN ai_costs.cached_tokens IS
    'Part of input_tokens served from the provider prefix cache (K3). Subset, not an addition. 0 = no hit or a provider that does not report one.';
