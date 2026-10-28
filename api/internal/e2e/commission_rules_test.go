package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Client-facing commission rules against a real database (§6.6, G1).
//
// These tests are about the two ways a commission module loses a customer's
// trust: paying somebody a number nobody can explain, and letting a rule that
// cannot have been meant reach the table.

func globalMarginRule() map[string]any {
	return map[string]any{
		"name":         "Padrão da casa",
		"scope":        "global",
		"base":         "margin",
		"rate_percent": 5,
	}
}

func TestCommissionRules_CRUDAndScopePrecedence(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	w := doRequest("POST", "/v1/commission-rules", globalMarginRule(), token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	created := parseJSON(t, w)
	ruleID, _ := created["id"].(string)
	require.NotEmpty(t, ruleID)
	assert.Equal(t, "margin", created["base"])
	assert.Equal(t, true, created["active"])

	// Only ONE active global rule: a second would make the fallback
	// ambiguous, and the newest-wins tie-break would silently change what
	// everybody gets paid the day somebody adds one "just to test".
	dup := doRequest("POST", "/v1/commission-rules", globalMarginRule(), token)
	assert.Equal(t, http.StatusConflict, dup.Code, dup.Body.String())

	// A seller rule coexists with the global one — the exception is how a
	// veteran on a different deal gets modelled without deleting anything.
	sellerRule := map[string]any{
		"name":         "Veterano",
		"scope":        "seller",
		"scope_id":     "01J000000000000000000SELLER",
		"base":         "gross",
		"rate_percent": 1.5,
	}
	sw := doRequest("POST", "/v1/commission-rules", sellerRule, token)
	require.Equal(t, http.StatusOK, sw.Code, sw.Body.String())

	assert.Len(t, parseJSONArray(t, doRequest("GET", "/v1/commission-rules", nil, token)), 2)

	// An update keeps the id and the creation date, and takes the id from the
	// PATH — a body pointing somewhere else must not redirect the write.
	upd := globalMarginRule()
	upd["rate_percent"] = 4
	upd["id"] = "01J0000000000000000000OTHER"
	uw := doRequest("PUT", "/v1/commission-rules/"+ruleID, upd, token)
	require.Equal(t, http.StatusOK, uw.Code, uw.Body.String())
	updated := parseJSON(t, uw)
	assert.Equal(t, ruleID, updated["id"])
	assert.EqualValues(t, 4, updated["rate_percent"])

	require.Equal(t, http.StatusNoContent,
		doRequest("DELETE", "/v1/commission-rules/"+ruleID, nil, token).Code)
	assert.Equal(t, http.StatusNotFound,
		doRequest("GET", "/v1/commission-rules/"+ruleID, nil, token).Code)
}

func TestCommissionRules_RefusesWhatNobodyCanHaveMeant(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	cases := []struct {
		name string
		body map[string]any
	}{
		{"rate above 100", map[string]any{
			"name": "Dedo escorregou", "scope": "global", "base": "margin",
			"rate_percent": 500,
		}},
		{"scoped rule pointing at nobody", map[string]any{
			"name": "Sem alvo", "scope": "seller", "base": "margin", "rate_percent": 3,
		}},
		{"global rule with a target", map[string]any{
			"name": "Global com alvo", "scope": "global", "scope_id": "algum-id",
			"base": "margin", "rate_percent": 3,
		}},
		{"splits that do not total 100", map[string]any{
			"name": "Split torto", "scope": "global", "base": "margin", "rate_percent": 5,
			"splits": []map[string]any{
				{"role": "closer", "share_percent": 60},
				{"role": "quem indicou", "share_percent": 20},
			},
		}},
		{"nameless", map[string]any{
			"scope": "global", "base": "margin", "rate_percent": 5,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequest("POST", "/v1/commission-rules", tc.body, token)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
	// Nothing was written: a refused rule must not leave a half-saved row
	// that pays somebody next month.
	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/commission-rules", nil, token)))
}

func TestCommissionRules_SimulateBeforeSaving(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	// R$ 10.000 a sale, 30% margin, 5% of margin → R$ 150 per sale.
	body := map[string]any{
		"rule": map[string]any{
			"name": "Simulação", "scope": "global", "base": "margin", "rate_percent": 5,
		},
		"gross_cents":     1000000,
		"margin_percent":  30,
		"sales_per_month": 8,
	}
	w := doRequest("POST", "/v1/commission-rules/simulate", body, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	sim := parseJSON(t, w)

	assert.EqualValues(t, 15000, sim["per_sale_cents"])
	assert.EqualValues(t, 120000, sim["monthly_cents"])
	// What the rule costs the company over the month's GROSS: 5% of a 30%
	// margin is 1.5% of revenue, and that is the number an owner needs.
	assert.EqualValues(t, 1.5, sim["effective_rate_percent"])
	calc, _ := sim["calc"].(map[string]any)
	require.NotNil(t, calc)
	assert.NotEmpty(t, calc["explanation"])

	// Simulating writes NOTHING — the whole point is trying a policy nobody
	// has committed to.
	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/commission-rules", nil, token)))

	// A gross-based rule on a thin margin is the trap the module exists to
	// expose: 5% of the invoice is a third of everything the deal made.
	body["rule"] = map[string]any{
		"name": "Sobre o bruto", "scope": "global", "base": "gross", "rate_percent": 5,
	}
	gw := doRequest("POST", "/v1/commission-rules/simulate", body, token)
	require.Equal(t, http.StatusOK, gw.Code, gw.Body.String())
	assert.EqualValues(t, 5, parseJSON(t, gw)["effective_rate_percent"])
}

func TestCommissionRules_TenancyAndRole(t *testing.T) {
	cleanDB(t)
	accountA := seedIdentityAccount(t)
	accountB := seedIdentityAccount(t)
	tokenA := tokenForSeededAccount(t, accountA, "admin")
	tokenB := tokenForSeededAccount(t, accountB, "admin")

	w := doRequest("POST", "/v1/commission-rules", globalMarginRule(), tokenA)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	ruleID, _ := parseJSON(t, w)["id"].(string)
	require.NotEmpty(t, ruleID)

	// Another company's pay policy is not readable, not writable, and not
	// deletable by guessing the id.
	assert.Equal(t, http.StatusNotFound,
		doRequest("GET", "/v1/commission-rules/"+ruleID, nil, tokenB).Code)
	upd := globalMarginRule()
	upd["rate_percent"] = 90
	assert.Equal(t, http.StatusNotFound,
		doRequest("PUT", "/v1/commission-rules/"+ruleID, upd, tokenB).Code)
	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/commission-rules", nil, tokenB)))

	// Reading a colleague's rate is reading their salary: the whole module is
	// admin-only, and the seller's own view is a narrower screen (G3).
	attendant := tokenForSeededAccount(t, accountA, "attendant")
	assert.Equal(t, http.StatusForbidden,
		doRequest("GET", "/v1/commission-rules", nil, attendant).Code)
	assert.Equal(t, http.StatusForbidden,
		doRequest("POST", "/v1/commission-rules", globalMarginRule(), attendant).Code)
}
