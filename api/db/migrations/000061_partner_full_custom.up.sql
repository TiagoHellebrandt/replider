-- Partner becomes a fully-custom contract plan, mirroring Enterprise.
--
-- Before this migration Partner had hardcoded caps (50 agents, 100
-- channels, 60K conv, 20M tokens, 2 TB) and a fixed R$ 5,490 price.
-- That made the landing card lie ("Custom" vs concrete DB numbers)
-- and forced every Partner deal into the same shape regardless of
-- contract.
--
-- New shape: zero everywhere. The IsContractPlan accessor on
-- `accounts` already treats `plan limit = 0 + no override = unlimited`
-- for contract plans (StripePriceID NULL covers Partner + Enterprise),
-- so the per-account override columns added in 000060 cover the
-- "white-label deal X gets 60K conv/mo, deal Y gets 200K" use case
-- without minting a new plans row per customer.
--
-- Token cap: BYOK is already enabled for Partner (SetAccountAPIKeysUseCase
-- gates on IsContractPlan, which is true for both contract plans), so
-- a Partner can either burn its own LLM key or have spend embedded in
-- the contract price.
UPDATE plans SET
  price                            = 0,
  max_agents                       = 0,
  max_channels                     = 0,
  max_seats                        = 0,
  max_conversations_per_month      = 0,
  max_ai_tokens_per_month          = 0,
  max_storage_mb                   = 0,
  max_broadcast_messages_per_month = 0,
  max_recipients_per_campaign      = 0,
  overage_conversation_cents       = 0,
  overage_cap_multiplier           = 0,
  updated_at                       = NOW()
WHERE id = '01PLAN_PARTNER_00000000';
