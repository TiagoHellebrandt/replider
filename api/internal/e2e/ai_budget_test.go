package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The BYOK cost surface: the spend report, the customer-set ceiling and the
// hard refusal once it is crossed.

func TestAIBudget_ReportAndCeilingRoundTrip(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	registerUser(t, "Corp", "budget@test.com", "pass1234")
	verifyEmail(t, "budget@test.com")
	token := login(t, "budget@test.com", "pass1234")

	// Fresh account: no history, no ceiling — honest zeros, never fake
	// projections.
	w := doRequest("GET", "/v1/me/ai-costs", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	resp := parseJSON(t, w)
	assert.Nil(t, resp["budget_cents"])
	assert.Equal(t, float64(0), resp["projected_monthly_brl_cents"])

	// Setting the ceiling echoes the fresh report.
	w = doRequest("PUT", "/v1/me/ai-budget", map[string]any{"budget_cents": 10000}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	resp = parseJSON(t, w)
	assert.Equal(t, float64(10000), resp["budget_cents"])

	var stored *int64
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT ai_monthly_budget_cents FROM accounts
		 WHERE id = (SELECT account_id FROM users WHERE email = 'budget@test.com')`).Scan(&stored))
	require.NotNil(t, stored)
	assert.Equal(t, int64(10000), *stored)

	// Zero clears it (no ceiling ≠ ceiling of zero).
	w = doRequest("PUT", "/v1/me/ai-budget", map[string]any{"budget_cents": 0}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT ai_monthly_budget_cents FROM accounts
		 WHERE id = (SELECT account_id FROM users WHERE email = 'budget@test.com')`).Scan(&stored))
	assert.Nil(t, stored)
}
