-- name: GetPlanByID :one
SELECT * FROM plans
WHERE id = $1 LIMIT 1;

-- name: ListAllPlans :many
SELECT * FROM plans
ORDER BY price ASC;

-- name: GetPlanByStripePriceID :many
-- Resolves the plan a Stripe price belongs to, matching either the
-- monthly or the annual price id. Price ids are unique across Stripe,
-- so a single match is the only correct outcome — LIMIT 2 lets the
-- repo detect a misconfigured price shared by two plans and refuse to
-- guess (US-023) instead of silently activating an arbitrary tier.
-- Used by the checkout webhook to activate the purchased plan.
SELECT * FROM plans
WHERE stripe_price_id = $1 OR stripe_price_id_annual = $1
LIMIT 2;
