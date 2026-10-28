package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The onboarding's last step over HTTP. The AI provider is not configured in
// e2e, so the draft itself cannot be written here — what these pin is the
// contract around it: an honest "nothing to offer" for an empty account, and
// that the customer path makes exactly ONE lead in the default funnel no
// matter how many times the step is reloaded.

func TestFirstOffer_AnEmptyAccountIsToldWhatToBring(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Cuiabamed", "dona@cuiabamed.test", "pass1234")
	verifyEmail(t, "dona@cuiabamed.test")
	token := login(t, "dona@cuiabamed.test", "pass1234")

	w := doRequest("POST", "/v1/me/onboarding/first-offer", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := parseJSON(t, w)
	assert.Equal(t, "nothing_to_offer", body["status"])
	assert.Equal(t, "no_customers_no_territory", body["reason"])
}

func TestFirstOffer_ACustomerBecomesOneLeadInTheDefaultFunnel_Idempotently(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Cuiabamed", "dona@cuiabamed.test", "pass1234")
	verifyEmail(t, "dona@cuiabamed.test")
	token := login(t, "dona@cuiabamed.test", "pass1234")

	me := parseJSON(t, doRequest("GET", "/v1/me", nil, token))
	accountID := (me["account"].(map[string]interface{}))["id"].(string)
	seedSaleContact(t, accountID)

	countLeads := func() int {
		var n int
		require.NoError(t, testPool.QueryRow(context.Background(),
			`SELECT count(*) FROM leads l JOIN sales_funnels f ON f.id = l.funnel_id
			 WHERE f.account_id = $1 AND f.is_default`, accountID).Scan(&n))
		return n
	}

	// The draft is refused AFTER the lead was made, and for the RIGHT
	// reason: this account imported no products yet. Until 2026-09-01 a
	// self-serve (trial) account was refused one step earlier with
	// AI_DRAFTS_NOT_ALLOWED — the trial plan carried a draft cap of zero —
	// which is exactly the door this test now keeps open.
	w := doRequest("POST", "/v1/me/onboarding/first-offer", nil, token)
	assert.NotEqual(t, http.StatusOK, w.Code, w.Body.String())
	assert.Less(t, w.Code, 500, w.Body.String())
	assert.Contains(t, w.Body.String(), "APPROACH_NEEDS_PRODUCTS",
		"a trial account must get past the draft cap; the only thing missing here is the catalogue")
	assert.NotContains(t, w.Body.String(), "AI_DRAFTS_NOT_ALLOWED")
	assert.Equal(t, 1, countLeads(), "the customer became a lead in the default funnel")

	// A reload does not make a second lead.
	doRequest("POST", "/v1/me/onboarding/first-offer", nil, token)
	assert.Equal(t, 1, countLeads())
}
