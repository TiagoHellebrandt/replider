-- Stop advertising a price that cannot be charged.
--
-- The three SDR tiers were seeded by 000154 with `is_listed = TRUE` and
-- `stripe_price_id = NULL`. HandleListPlans filters only on is_listed, so all
-- three reach the public catalogue with their prices — and every payment path
-- then refuses them, correctly:
--
--   CreateCheckoutSessionUseCase   → ErrPlanNotPurchasable (create_checkout_session.go)
--   CreateTransparentCheckoutUseCase → the same refusal on the BRL path
--
-- So the product showed a price and a subscribe button that could not work. The
-- first customers are being billed outside the product, so the honest state is
-- unlisted: no Stripe Price is configured here on purpose.
--
-- Unlisted, NOT deleted — the same treatment 000154 gave the retired attendance
-- ladder. GetPlanByID stays unfiltered, so an account already assigned one of
-- these tiers (an owner-set contract) keeps working exactly as before; this only
-- stops the catalogue from offering them.
--
-- To sell them self-serve later: create the Stripe Prices, write them into
-- stripe_price_id (and stripe_price_id_annual if annual is offered), and flip
-- is_listed back to TRUE in the SAME migration. Flipping is_listed without the
-- price ids puts this exact bug back.
UPDATE plans
SET is_listed = FALSE, updated_at = NOW()
WHERE id IN (
  '01PLAN_SDR_VENDEDOR_000',
  '01PLAN_SDR_EQUIPE_00000',
  '01PLAN_SDR_ESCALA_00000'
);
