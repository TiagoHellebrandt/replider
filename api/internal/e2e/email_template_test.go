package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A library of short texts with variables, not a drag-and-drop builder. The
// editor everyone asks for produces exactly the emails that land in spam.

func TestEmailTemplate_CreateReadUpdate(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	w := doRequest("PUT", "/v1/email-templates/new", map[string]any{
		"name":    "Primeira abordagem",
		"subject": "Monitores para a {{empresa}}",
		"body":    "Olá {{nome}}, trabalho com monitores multiparâmetro.",
	}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	created := parseJSON(t, w)
	id := created["id"].(string)
	// The variables come from the server so the editor cannot compute a
	// different answer from the renderer.
	vars, _ := created["variables"].([]any)
	assert.ElementsMatch(t, []any{"empresa", "nome"}, vars)
	assert.Equal(t, float64(1), created["version"])

	list := parseJSONArray(t, doRequest("GET", "/v1/email-templates", nil, token))
	require.Len(t, list, 1)

	// Editing the TEXT earns a new version, because "12% de resposta" is
	// meaningless if the words changed since the number was earned.
	upd := parseJSON(t, doRequest("PUT", "/v1/email-templates/"+id, map[string]any{
		"name":    "Primeira abordagem",
		"subject": "Monitores para a {{empresa}}",
		"body":    "Texto novo, {{nome}}.",
	}, token))
	assert.Equal(t, float64(2), upd["version"])
}

// Renaming must not orphan the numbers the text earned.
func TestEmailTemplate_RenamingKeepsTheVersion(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	created := parseJSON(t, doRequest("PUT", "/v1/email-templates/new", map[string]any{
		"name": "Nome antigo", "subject": "Assunto", "body": "Corpo",
	}, token))
	id := created["id"].(string)

	renamed := parseJSON(t, doRequest("PUT", "/v1/email-templates/"+id, map[string]any{
		"name": "Nome novo", "subject": "Assunto", "body": "Corpo",
	}, token))

	assert.Equal(t, "Nome novo", renamed["name"])
	assert.Equal(t, float64(1), renamed["version"])
}

// A long body is flagged, not refused: the person writing knows their market,
// and the hundred-word line is a rule of thumb.
func TestEmailTemplate_LongBodyIsWarnedNotBlocked(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	long := ""
	for i := 0; i < 150; i++ {
		long += "palavra "
	}
	got := parseJSON(t, doRequest("PUT", "/v1/email-templates/new", map[string]any{
		"name": "Longo", "body": long,
	}, token))

	assert.Equal(t, true, got["too_long"])
	assert.Equal(t, float64(150), got["word_count"])
}

// The upsert is keyed by id alone, so without an ownership check a guessed id
// would create a row under the wrong owner.
func TestEmailTemplate_CannotWriteOverAnotherAccountsTemplate(t *testing.T) {
	cleanDB(t)
	accountA := seedIdentityAccount(t)
	accountB := seedIdentityAccount(t)

	created := parseJSON(t, doRequest("PUT", "/v1/email-templates/new",
		map[string]any{"name": "Da conta A", "body": "Corpo"},
		tokenForSeededAccount(t, accountA, "admin")))
	id := created["id"].(string)

	w := doRequest("PUT", "/v1/email-templates/"+id,
		map[string]any{"name": "Sequestrado", "body": "Outro"},
		tokenForSeededAccount(t, accountB, "admin"))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Retiring is soft: the stats reference the template, and a hard delete would
// erase the evidence for every campaign that used it.
func TestEmailTemplate_DeleteHidesItWithoutLosingIt(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	created := parseJSON(t, doRequest("PUT", "/v1/email-templates/new",
		map[string]any{"name": "Temporário", "body": "Corpo"}, token))
	id := created["id"].(string)

	require.Equal(t, http.StatusNoContent,
		doRequest("DELETE", "/v1/email-templates/"+id, nil, token).Code)

	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/email-templates", nil, token)))
	// Still readable by id — the row is there, carrying its numbers.
	assert.Equal(t, http.StatusOK,
		doRequest("GET", "/v1/email-templates/"+id, nil, token).Code)
}
