package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The company standardises, the salesperson personalises, compliance never
// bends. These pin the third clause — the other two are style, and this one
// is regulatory.

func TestBrandKit_EmptyAccountGetsAFormNotA404(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	w := doRequest("GET", "/v1/brand-kit", nil, token)

	require.Equal(t, http.StatusOK, w.Code)
	body := parseJSON(t, w)
	assert.Equal(t, "", body["company_name"])
	// A screen that has to special-case "not created yet" gets that case
	// wrong; empty arrays let the form just render.
	assert.NotNil(t, body["forbidden_claims"])
}

func TestBrandKit_RoundTrips(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	w := doRequest("PUT", "/v1/brand-kit", map[string]any{
		"company_name":     "Distribuidora Exemplo",
		"tone_of_voice":    "Direto, sem jargão.",
		"forbidden_claims": []string{"cura", "aprovado pela ANVISA", "   "},
	}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got := parseJSON(t, doRequest("GET", "/v1/brand-kit", nil, token))
	assert.Equal(t, "Distribuidora Exemplo", got["company_name"])
	claims, _ := got["forbidden_claims"].([]any)
	// The blank row is dropped: a forbidden claim of "" would match every
	// text ever written.
	assert.Len(t, claims, 2)
}

// Deciding what the BUSINESS may claim is not one salesperson's call.
func TestBrandKit_OnlyAdminsCanChangeWhatTheCompanyMayClaim(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	attendant := tokenForSeededAccount(t, accountID, "attendant")

	w := doRequest("PUT", "/v1/brand-kit", map[string]any{
		"forbidden_claims": []string{},
	}, attendant)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// But reading it is not an admin act: a salesperson needs to know what they
// may and may not claim.
func TestBrandKit_ASalespersonCanReadTheRules(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	attendant := tokenForSeededAccount(t, accountID, "attendant")

	assert.Equal(t, http.StatusOK, doRequest("GET", "/v1/brand-kit", nil, attendant).Code)
}

// The overlay is scoped to the caller by the token, never by the body, so it
// cannot be used to rewrite a colleague's signature.
func TestSellerOverlay_IsScopedToTheCaller(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	a := tokenForSeededAccount(t, accountID, "attendant")
	b := tokenForSeededAccount(t, accountID, "attendant")

	require.Equal(t, http.StatusOK, doRequest("PUT", "/v1/me/seller-overlay",
		map[string]any{"signature": "João, 15 99999-0000"}, a).Code)

	// The other seller's overlay is untouched.
	other := parseJSON(t, doRequest("GET", "/v1/me/seller-overlay", nil, b))
	assert.Equal(t, "", other["signature"])

	mine := parseJSON(t, doRequest("GET", "/v1/me/seller-overlay", nil, a))
	assert.Equal(t, "João, 15 99999-0000", mine["signature"])
}

// One account's brand kit must never be readable by another.
func TestBrandKit_IsScopedToTheAccount(t *testing.T) {
	cleanDB(t)
	accountA := seedIdentityAccount(t)
	accountB := seedIdentityAccount(t)

	require.Equal(t, http.StatusOK, doRequest("PUT", "/v1/brand-kit",
		map[string]any{"company_name": "Empresa A"},
		tokenForSeededAccount(t, accountA, "admin")).Code)

	got := parseJSON(t, doRequest("GET", "/v1/brand-kit", nil,
		tokenForSeededAccount(t, accountB, "admin")))
	assert.Equal(t, "", got["company_name"])
}
