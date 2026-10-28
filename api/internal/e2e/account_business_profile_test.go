package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// What the company does and where: PATCH /accounts stores it, GET /me hands
// it back, and the DB refuses a line the product does not know — the three
// contracts the onboarding's first screen depends on.
func TestAccountBusinessProfile_RoundTrip(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Cuiabamed", "dona@cuiabamed.test", "pass1234")
	verifyEmail(t, "dona@cuiabamed.test")
	token := login(t, "dona@cuiabamed.test", "pass1234")

	// Fresh account: nothing said yet, and the shape is still an array.
	me := parseJSON(t, doRequest("GET", "/v1/me", nil, token))
	acc := me["account"].(map[string]interface{})
	assert.Equal(t, []interface{}{}, acc["business_lines"])
	assert.Nil(t, acc["territory"])

	w := doRequest("PATCH", "/v1/accounts", map[string]interface{}{
		"business_lines": []string{"service", "sell", "rent"},
		"territory":      map[string]interface{}{"uf": "mt", "cities": []string{"Cuiabá", "Várzea Grande"}},
	}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	me = parseJSON(t, doRequest("GET", "/v1/me", nil, token))
	acc = me["account"].(map[string]interface{})
	assert.Equal(t, []interface{}{"service", "sell", "rent"}, acc["business_lines"])
	terr := acc["territory"].(map[string]interface{})
	assert.Equal(t, "MT", terr["uf"])
	assert.Equal(t, []interface{}{"Cuiabá", "Várzea Grande"}, terr["cities"])
}

func TestAccountBusinessProfile_RefusesAnUnknownLine(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Cuiabamed", "dona@cuiabamed.test", "pass1234")
	verifyEmail(t, "dona@cuiabamed.test")
	token := login(t, "dona@cuiabamed.test", "pass1234")

	w := doRequest("PATCH", "/v1/accounts", map[string]interface{}{
		"business_lines": []string{"consulting"},
	}, token)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// The constraint is in the database too, so a path that skips the use case
// (a hand-run SQL, a future import) cannot store a line nobody reads.
func TestAccountBusinessProfile_TheDatabaseRefusesAnUnknownLineToo(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	_, err := testPool.Exec(t.Context(),
		`UPDATE accounts SET business_lines = ARRAY['consulting'] WHERE id = $1`, accountID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accounts_business_lines_known")
}
