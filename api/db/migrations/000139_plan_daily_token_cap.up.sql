-- Per-plan daily AI token cap (review-fixes US-016). The domain field
-- Plan.MaxDailyAITokens existed since US-025 but had no column — every
-- plan silently fell back to the 100K platform default, which capped
-- USD Pro (105M tokens/month marketed) at ~3M/month in practice: a
-- paying customer could never reach the conversation allowance the
-- pricing page sells.
--
-- Sizing: 1/5 of the monthly allowance per day. A single runaway day
-- is still bounded to 20% of the franchise (the cap's anti-abuse
-- purpose), while 30 × daily = 6× monthly, so the daily cap can never
-- throttle an account below its marketed monthly allowance. Floored
-- at the 100K platform default so small tiers don't regress below
-- their previous effective cap.
--
-- Plans with max_ai_tokens_per_month = 0 (full-custom contract, e.g.
-- Partner) keep 0 = "use the platform default" (see
-- Plan.EffectiveMaxDailyAITokens). Trial has no plans row — its cap
-- stays the in-code default, deliberately low.
ALTER TABLE plans ADD COLUMN max_daily_ai_tokens BIGINT NOT NULL DEFAULT 0;

UPDATE plans
SET max_daily_ai_tokens = GREATEST(CEIL(max_ai_tokens_per_month / 5.0)::BIGINT, 100000),
    updated_at = NOW()
WHERE max_ai_tokens_per_month > 0;
