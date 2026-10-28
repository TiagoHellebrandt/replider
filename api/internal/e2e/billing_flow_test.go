package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_ListPlans(t *testing.T) {
	cleanDB(t)

	// GET /plans is public (no auth needed)
	w := doRequest("GET", "/v1/plans", nil, "")
	assert.Equal(t, 200, w.Code, "list plans failed: %s", w.Body.String())

	plans := parseJSONArray(t, w)
	names := make([]string, len(plans))
	for i, p := range plans {
		plan := p.(map[string]interface{})
		names[i] = plan["name"].(string)
	}
	// The BRL catalogue is contact-sales only (migration 000197). The three SDR
	// tiers were listed with a price and NO stripe_price_id, so every checkout
	// path refused them — the product advertised a number nobody could be
	// charged. They are unlisted until the Stripe Prices exist; the first
	// customers are billed outside the product.
	assert.Contains(t, names, "Enterprise")
	for _, unlisted := range []string{"Vendedor", "Equipe", "Escala"} {
		assert.NotContains(t, names, unlisted,
			"an SDR tier must stay out of the catalogue until it has a Stripe price")
	}
	assert.NotContains(t, names, "Partner", "retired Partner plan must stay hidden from the catalogue")
	// The attendance ladder is retired, not deleted — hidden from the
	// catalogue so nobody new subscribes to it.
	for _, retired := range []string{"Solo", "Starter", "Pro", "Business", "Scale"} {
		assert.NotContains(t, names, retired, "the attendance ladder must be unlisted")
	}
}

// The invariant behind migration 000197, stated as a rule instead of a list.
//
// A plan reaches the public catalogue through is_listed alone, and a listed plan
// with a price but no stripe_price_id is a price nobody can be charged:
// CreateCheckoutSessionUseCase and CreateTransparentCheckoutUseCase both refuse
// it with PLAN_NOT_PURCHASABLE. Zero-price contract tiers (Enterprise) are the
// deliberate exception — the app renders those as "Sob consulta" with a
// contact-sales CTA and never offers a subscribe button.
//
// Written against the TABLE rather than the endpoint because it has to hold for
// every currency, and /v1/plans only ever answers for one.
func TestE2E_NoListedPlanAdvertisesAPriceItCannotCharge(t *testing.T) {
	rows, err := testPool.Query(context.Background(),
		`SELECT id, name, currency, price
		   FROM plans
		  WHERE is_listed = TRUE AND price > 0
		    AND (stripe_price_id IS NULL OR stripe_price_id = '')`)
	require.NoError(t, err)
	defer rows.Close()

	var offenders []string
	for rows.Next() {
		var id, name, currency string
		var price int64
		require.NoError(t, rows.Scan(&id, &name, &currency, &price))
		offenders = append(offenders, id+" ("+name+", "+currency+")")
	}
	require.NoError(t, rows.Err())

	assert.Empty(t, offenders,
		"these plans are advertised at a price every checkout path refuses; "+
			"either seed their Stripe Price or set is_listed = FALSE: %v", offenders)
}

// Retired does NOT mean gone: an account already on an old plan must keep
// working, so the plan stays fetchable by id even though the catalogue
// hides it. Unlisting a plan that customers are paying for and then 404ing
// it would break their billing, not just their upgrade page.
func TestE2E_RetiredPlansStayResolvableByID(t *testing.T) {
	cleanDB(t)

	var name string
	var listed bool
	err := testPool.QueryRow(context.Background(),
		`SELECT name, is_listed FROM plans WHERE id = '01PLAN_BRONZE_000000000'`).
		Scan(&name, &listed)
	require.NoError(t, err)
	assert.Equal(t, "Starter", name)
	assert.False(t, listed, "the retired plan is hidden from the catalogue")
}

func TestE2E_AccountUsage(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Corp", "admin@test.com", "pass1234")
	verifyEmail(t, "admin@test.com")
	token := login(t, "admin@test.com", "pass1234")

	// GET /accounts/usage
	w := doRequest("GET", "/v1/accounts/usage", nil, token)
	assert.Equal(t, 200, w.Code, "get usage failed: %s", w.Body.String())

	resp := parseJSON(t, w)
	// Usage should exist (created during registration)
	assert.NotNil(t, resp)
}

func TestE2E_AddonPacks(t *testing.T) {
	// GET /payments/addon-packs is public
	w := doRequest("GET", "/v1/payments/addon-packs", nil, "")
	assert.Equal(t, 200, w.Code, "list addon packs failed: %s", w.Body.String())

	packs := parseJSONArray(t, w)
	assert.Greater(t, len(packs), 0, "expected at least one addon pack")
}
