package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
)

// Machine API keys against a real database (§6.19, J1).
//
// What only a real request proves: the plaintext key authenticates, a revoked one
// stops immediately, and the scope on the key actually gates the route — a key
// that authenticates but is not scope-checked is worse than no key at all.

func createAPIKey(t *testing.T, token string, scopes ...string) (id, plaintext string) {
	t.Helper()
	w := doRequest("POST", "/v1/api-keys", map[string]any{
		"name": "ERP Protheus", "scopes": scopes,
	}, token)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	body := parseJSON(t, w)

	key, _ := body["key"].(string)
	require.True(t, strings.HasPrefix(key, domain.TokenPrefixAPIKey),
		"the key must carry its own prefix, got %q", key)
	keyID, _ := body["id"].(string)
	return keyID, key
}

func TestAPIKey_AuthenticatesAndIsScopeGated(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	funnelID := seedCCFunnel(t, accountID)
	seedExportLead(t, accountID, funnelID, "new", "pncp")

	_, key := createAPIKey(t, token, "export:read")

	// The key authenticates the public surface.
	w := doRequest("GET", "/v1/public/export/leads", nil, key)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Len(t, exportCSVLines(t, w.Body.String()), 2)

	// And the scope actually gates: no analytics:read on this key.
	denied := doRequest("GET", "/v1/public/analytics/sdr", nil, key)
	assert.Equal(t, http.StatusForbidden, denied.Code, denied.Body.String())
	// The refusal names the scope it wanted, so the fix is one line.
	assert.Contains(t, denied.Body.String(), "analytics:read")
}

func TestAPIKey_TheSecretIsShownOnceAndNeverStored(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	_, key := createAPIKey(t, token, "leads:read")

	// The listing never carries it back.
	list := doRequest("GET", "/v1/api-keys", nil, token)
	require.Equal(t, http.StatusOK, list.Code)
	assert.NotContains(t, list.Body.String(), key)

	// And the plaintext is nowhere in the database — only its hash.
	var stored int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM api_tokens WHERE token_hash = $1`,
		domain.HashToken(key)).Scan(&stored))
	assert.Equal(t, 1, stored)

	var plaintextRows int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM api_tokens WHERE token_hash = $1`, key).Scan(&plaintextRows))
	assert.Equal(t, 0, plaintextRows, "the plaintext must never be stored")
}

func TestAPIKey_RevokingStopsItImmediately(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	seedCCFunnel(t, accountID)

	id, key := createAPIKey(t, token, "export:read")
	require.Equal(t, http.StatusOK,
		doRequest("GET", "/v1/public/export/leads", nil, key).Code)

	revoked := doRequest("DELETE", "/v1/api-keys/"+id, nil, token)
	require.Equal(t, http.StatusNoContent, revoked.Code, revoked.Body.String())

	// A leaked key has to die the moment somebody says so.
	after := doRequest("GET", "/v1/public/export/leads", nil, key)
	assert.Equal(t, http.StatusUnauthorized, after.Code)

	// Revoking again reports 404 rather than a success that did nothing —
	// somebody killing a leaked credential needs to know it happened.
	again := doRequest("DELETE", "/v1/api-keys/"+id, nil, token)
	assert.Equal(t, http.StatusNotFound, again.Code)
}

func TestAPIKey_RefusesWhatItCannotHonour(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	noScopes := doRequest("POST", "/v1/api-keys",
		map[string]any{"name": "x", "scopes": []string{}}, token)
	assert.Equal(t, http.StatusBadRequest, noScopes.Code, noScopes.Body.String())

	unknown := doRequest("POST", "/v1/api-keys",
		map[string]any{"name": "x", "scopes": []string{"leads:delete"}}, token)
	assert.Equal(t, http.StatusBadRequest, unknown.Code)
	assert.Contains(t, unknown.Body.String(), "leads:delete")
}

func TestAPIKey_AUserJWTIsNotAMachineKey(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	// Pasting the dashboard session into a machine integration must fail as
	// "wrong kind of credential", not as a mysterious 401 hours later.
	w := doRequest("GET", "/v1/public/export/leads", nil, token)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "API_KEY_REQUIRED")
}

func TestAPIKey_LeaksNothingAcrossAccounts(t *testing.T) {
	cleanDB(t)
	mine := seedIdentityAccount(t)
	myToken := tokenForSeededAccount(t, mine, "admin")
	myFunnel := seedCCFunnel(t, mine)
	seedExportLead(t, mine, myFunnel, "new", "pncp")

	theirs := seedIdentityAccount(t)
	theirToken := tokenForSeededAccount(t, theirs, "admin")
	theirFunnel := seedCCFunnel(t, theirs)
	for i := 0; i < 3; i++ {
		seedExportLead(t, theirs, theirFunnel, "new", "pncp")
	}

	_, myKey := createAPIKey(t, myToken, "export:read")

	// The key resolves to ITS account, whatever it asks for.
	w := doRequest("GET", "/v1/public/export/leads", nil, myKey)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, exportCSVLines(t, w.Body.String()), 2)

	// And another tenant's key id cannot be revoked from here.
	theirID, _ := createAPIKey(t, theirToken, "export:read")
	cross := doRequest("DELETE", "/v1/api-keys/"+theirID, nil, myToken)
	assert.Equal(t, http.StatusNotFound, cross.Code)
}

func TestAPIKey_TheKeyIsTheAccountsNotThePersons(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	id, _ := createAPIKey(t, token, "leads:read")

	// The creator is recorded — that is an audit fact — but the key belongs to
	// the account and outlives them, which is the entire reason it exists.
	var storedAccount string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT account_id FROM api_tokens WHERE id = $1`, id).Scan(&storedAccount))
	assert.Equal(t, accountID, storedAccount)
}

func TestAPIKey_RepositoryRefusesAnExpiredKey(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	_, key := createAPIKey(t, token, "leads:read")

	_, err := testPool.Exec(ctx,
		`UPDATE api_tokens SET expires_at = NOW() - interval '1 hour'
		 WHERE token_hash = $1`, domain.HashToken(key))
	require.NoError(t, err)

	repo := postgres.NewAPIKeyRepository(testPool)
	resolved, err := repo.ResolveKey(ctx, domain.HashToken(key))
	require.NoError(t, err)
	// One answer for expired, revoked and never-existed: distinguishing them
	// would make this an oracle.
	assert.Nil(t, resolved)
}
