package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Audiences against a real database: the CRUD, and the one rule with teeth —
// editing a suggestion makes it yours.

func TestAudiences_CreateListDelete(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	created := parseJSON(t, doRequest("PUT", "/v1/audiences/new", map[string]any{
		"segment":        "clínicas de diálise",
		"size":           "10 a 50 leitos",
		"decision_maker": "diretor clínico",
		"regions":        []string{"Sorocaba"},
	}, token))
	id := created["id"].(string)
	require.NotEmpty(t, id)
	// A nameless audience is unpickable in a dropdown; the segment is what
	// the person would have called it anyway.
	assert.Equal(t, "clínicas de diálise", created["name"])

	list := parseJSONArray(t, doRequest("GET", "/v1/audiences", nil, token))
	require.Len(t, list, 1)

	require.Equal(t, http.StatusNoContent,
		doRequest("DELETE", "/v1/audiences/"+id, nil, token).Code)
	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/audiences", nil, token)))
}

// Keywords are what actually reaches the prospecting source, and they are
// derived rather than demanded — the role is deliberately left out, because
// Google Places indexes businesses and "gerente de compras" finds recruiters.
func TestAudiences_KeywordsAreDerivedFromTheDescription(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	created := parseJSON(t, doRequest("PUT", "/v1/audiences/new", map[string]any{
		"segment":        "clínicas de diálise",
		"decision_maker": "gerente de compras",
	}, token))

	keywords, _ := created["keywords"].([]any)
	assert.Equal(t, []any{"clínicas de diálise"}, keywords)
}

func TestAudiences_RefuseAnEmptyOne(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	w := doRequest("PUT", "/v1/audiences/new", map[string]any{}, token)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// One account's audiences are not another's.
func TestAudiences_AreScopedToTheAccount(t *testing.T) {
	cleanDB(t)
	owner := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")
	require.Equal(t, http.StatusOK, doRequest("PUT", "/v1/audiences/new",
		map[string]any{"segment": "hospitais"}, owner).Code)

	other := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/audiences", nil, other)))
}

// A suggestion invented out of an empty brand kit is a guess wearing a
// recommendation's clothes, and saying so is better than producing one.
func TestAudiences_SuggestRefusesWithNothingToReasonFrom(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	w := doRequest("POST", "/v1/audiences/suggest", nil, token)

	// The suite wires no AI provider, so this lands on one of the two honest
	// refusals — never on an invented list.
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "AUDIENCE_")
}
