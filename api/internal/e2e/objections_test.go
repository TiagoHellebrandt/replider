package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Objections v1: the library endpoints and the seed fallback.

func TestObjections_SeedThenOwnLibraryThenReset(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Corp", "obj@test.com", "pass1234")
	verifyEmail(t, "obj@test.com")
	token := login(t, "obj@test.com", "pass1234")

	// A fresh account reads the vertical seed, flagged as such.
	w := doRequest("GET", "/v1/objections", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	resp := parseJSON(t, w)
	assert.Equal(t, true, resp["is_default"])
	items := resp["items"].([]interface{})
	require.NotEmpty(t, items)
	first := items[0].(map[string]interface{})
	assert.Equal(t, "Já temos fornecedor", first["label"])

	// Saving a library replaces the seed.
	w = doRequest("PUT", "/v1/objections", map[string]any{
		"items": []map[string]string{
			{"label": "Só compramos por licitação", "counter": "Entramos no próximo pregão."},
			{"label": "   ", "counter": "blank label is dropped"},
		},
	}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	resp = parseJSON(t, w)
	assert.Equal(t, false, resp["is_default"])
	items = resp["items"].([]interface{})
	require.Len(t, items, 1)

	// Saving an empty list returns to the seed (reset without a second
	// endpoint).
	w = doRequest("PUT", "/v1/objections", map[string]any{"items": []map[string]string{}}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	resp = parseJSON(t, w)
	assert.Equal(t, true, resp["is_default"])
}
