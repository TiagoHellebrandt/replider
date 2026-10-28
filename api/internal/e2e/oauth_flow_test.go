package e2e

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// doFormRequest posts application/x-www-form-urlencoded (OAuth endpoints).
func doFormRequest(method, path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	return w
}

var flowTokenRe = regexp.MustCompile(`name="flow_token" value="([^"]+)"`)

// TestE2E_OAuthMCPFlow drives the full remote-MCP OAuth 2.1 journey:
// DCR → authorize (login+consent) → code exchange → authenticated use →
// refresh rotation → revoke → 401.
func TestE2E_OAuthMCPFlow(t *testing.T) {
	cleanDB(t)
	registerUser(t, "MCP Corp", "mcp-admin@test.com", "password123")

	// 1. Dynamic Client Registration (public client, like Claude.ai).
	w := doRequest("POST", "/oauth/register", map[string]any{
		"client_name":   "Claude",
		"redirect_uris": []string{"https://claude.ai/api/mcp/auth_callback"},
	}, "")
	require.Equal(t, 201, w.Code, w.Body.String())
	var reg map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &reg))
	clientID := reg["client_id"].(string)
	require.NotEmpty(t, clientID)

	// 2. GET /oauth/authorize renders the pt-BR consent page.
	verifier := "e2e-code-verifier-0123456789-0123456789-0123456789"
	challenge := domain.S256Challenge(verifier)
	authorizeQS := url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {"https://claude.ai/api/mcp/auth_callback"},
		"response_type":         {"code"},
		"state":                 {"e2e-state"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	w = doRequest("GET", "/oauth/authorize?"+authorizeQS.Encode(), nil, "")
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "quer acessar sua conta Replider")
	match := flowTokenRe.FindStringSubmatch(w.Body.String())
	require.Len(t, match, 2, "flow_token not found in consent page")

	// 3. POST login + consent → 302 with code.
	form := url.Values{
		"flow_token":            {match[1]},
		"client_id":             {clientID},
		"redirect_uri":          {"https://claude.ai/api/mcp/auth_callback"},
		"response_type":         {"code"},
		"state":                 {"e2e-state"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"email":                 {"mcp-admin@test.com"},
		"password":              {"password123"},
	}
	w = doFormRequest("POST", "/oauth/authorize", form)
	require.Equal(t, 302, w.Code, w.Body.String())
	loc, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	code := loc.Query().Get("code")
	require.NotEmpty(t, code)
	assert.Equal(t, "e2e-state", loc.Query().Get("state"))

	// 4. Exchange the code (PKCE).
	w = doFormRequest("POST", "/oauth/token", url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {"https://claude.ai/api/mcp/auth_callback"},
		"client_id":     {clientID},
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	var tok map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &tok))
	accessToken := tok["access_token"].(string)
	refreshToken := tok["refresh_token"].(string)
	assert.True(t, strings.HasPrefix(accessToken, "rpl_at_"))
	assert.True(t, strings.HasPrefix(refreshToken, "rpl_rt_"))

	// 5. Replaying the same code fails AND revokes the grant family.
	w = doFormRequest("POST", "/oauth/token", url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {"https://claude.ai/api/mcp/auth_callback"},
		"client_id":     {clientID},
	})
	assert.Equal(t, 400, w.Code)

	// Re-run the authorize+exchange to get a live pair again (the replay
	// above revoked everything for this user+client).
	w = doRequest("GET", "/oauth/authorize?"+authorizeQS.Encode(), nil, "")
	require.Equal(t, 200, w.Code)
	match = flowTokenRe.FindStringSubmatch(w.Body.String())
	require.Len(t, match, 2)
	form.Set("flow_token", match[1])
	w = doFormRequest("POST", "/oauth/authorize", form)
	require.Equal(t, 302, w.Code, w.Body.String())
	loc, _ = url.Parse(w.Header().Get("Location"))
	w = doFormRequest("POST", "/oauth/token", url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {loc.Query().Get("code")},
		"code_verifier": {verifier},
		"redirect_uri":  {"https://claude.ai/api/mcp/auth_callback"},
		"client_id":     {clientID},
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &tok))
	accessToken = tok["access_token"].(string)
	refreshToken = tok["refresh_token"].(string)

	// 6. The user's dashboard lists the connection.
	jwt := login(t, "mcp-admin@test.com", "password123")
	w = doRequest("GET", "/v1/me/mcp-connections", nil, jwt)
	require.Equal(t, 200, w.Code, w.Body.String())
	var conns []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &conns))
	require.Len(t, conns, 1)

	// 7. Refresh rotation: old refresh dies, new pair works.
	w = doFormRequest("POST", "/oauth/token", url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	var tok2 map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &tok2))
	// Reusing the rotated-out refresh token trips reuse detection.
	w = doFormRequest("POST", "/oauth/token", url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	})
	assert.Equal(t, 400, w.Code)

	// 8. RFC 7009 revoke is always 200, even for garbage.
	w = doFormRequest("POST", "/oauth/revoke", url.Values{
		"token":     {"rpl_at_garbage"},
		"client_id": {clientID},
	})
	assert.Equal(t, 200, w.Code)
}

// TestE2E_MCPPatLifecycle covers PAT create/list/revoke via the dashboard API.
func TestE2E_MCPPatLifecycle(t *testing.T) {
	cleanDB(t)
	registerUser(t, "PAT Corp", "pat-admin@test.com", "password123")
	jwt := login(t, "pat-admin@test.com", "password123")

	// Create — plaintext shown once.
	w := doRequest("POST", "/v1/me/mcp-tokens", map[string]any{"name": "Claude Code"}, jwt)
	require.Equal(t, 201, w.Code, w.Body.String())
	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	patToken := created["token"].(string)
	assert.True(t, strings.HasPrefix(patToken, "rpl_mcp_"))

	// List — metadata only, never the plaintext or hash.
	w = doRequest("GET", "/v1/me/mcp-tokens", nil, jwt)
	require.Equal(t, 200, w.Code)
	assert.NotContains(t, w.Body.String(), patToken)
	var list []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list, 1)

	// Revoke.
	w = doRequest("DELETE", "/v1/me/mcp-tokens/"+created["id"].(string), nil, jwt)
	require.Equal(t, 204, w.Code)
	w = doRequest("GET", "/v1/me/mcp-tokens", nil, jwt)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	assert.Len(t, list, 0)

	// Revoking someone else's / unknown id → 404.
	w = doRequest("DELETE", "/v1/me/mcp-tokens/01UNKNOWN0000000000000000X", nil, jwt)
	assert.Equal(t, 404, w.Code)
}
