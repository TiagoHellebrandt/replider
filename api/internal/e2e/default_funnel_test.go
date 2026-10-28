package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every account has one funnel it never had to create. The Clientes tab
// reads GET /funnels, and until 2026-08-18 a fresh account read an empty
// list and was told "you have no funnel yet" — a sentence about a concept
// the product no longer asks anyone to understand.

func TestDefaultFunnel_ANewAccountAlreadyHasOne(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Cuiabamed", "dona@cuiabamed.test", "pass1234")
	verifyEmail(t, "dona@cuiabamed.test")
	token := login(t, "dona@cuiabamed.test", "pass1234")

	list := parseJSONArray(t, doRequest("GET", "/v1/funnels", nil, token))
	require.Len(t, list, 1, "a fresh account must have exactly its default funnel")
	f := list[0].(map[string]interface{})
	assert.Equal(t, "Clientes", f["name"])
	assert.Equal(t, true, f["is_default"])
}

func TestDefaultFunnel_CannotBeDeleted(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Cuiabamed", "dona@cuiabamed.test", "pass1234")
	verifyEmail(t, "dona@cuiabamed.test")
	token := login(t, "dona@cuiabamed.test", "pass1234")

	list := parseJSONArray(t, doRequest("GET", "/v1/funnels", nil, token))
	require.Len(t, list, 1)
	id := list[0].(map[string]interface{})["id"].(string)

	w := doRequest("DELETE", "/v1/funnels/"+id, nil, token)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())

	// An ordinary funnel still goes.
	extra := createFunnel(t, token, "Pregões")
	w = doRequest("DELETE", "/v1/funnels/"+extra, nil, token)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// An account that predates the default (or lost it to a hand-run SQL) gets
// one on the first read — the list self-heals rather than showing empty.
func TestDefaultFunnel_IsRecreatedOnReadWhenMissing(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t) // raw INSERT: no funnel at all
	token := tokenForSeededAccount(t, accountID, "admin")

	list := parseJSONArray(t, doRequest("GET", "/v1/funnels", nil, token))
	require.Len(t, list, 1)
	assert.Equal(t, true, list[0].(map[string]interface{})["is_default"])

	// And a second read does not make a second one.
	again := parseJSONArray(t, doRequest("GET", "/v1/funnels", nil, token))
	assert.Len(t, again, 1)
}
