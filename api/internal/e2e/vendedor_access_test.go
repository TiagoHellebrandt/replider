package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// What a vendedor (and a gestor) can reach over HTTP.
//
// The Clientes tab starts by reading GET /funnels — registered under
// RequireRole("admin") until now, so the whole tab answered 403 for the two
// roles that live in it. And the timeline let a vendedor READ the reply to her
// proposal while every place to ANSWER it was admin/attendant. Both holes were
// in the router, not in the use cases, so these run over HTTP.

func TestVendedorAccess_FunnelListIsReadableByVendedorAndGestor(t *testing.T) {
	f := newRBACFixture(t)

	for _, tc := range []struct{ user, role string }{
		{f.vendedorA, "vendedor"},
		{f.gestor, "gestor"},
	} {
		token := tokenForUser(t, f.accountID, tc.user, tc.role)
		rec := doRequest("GET", "/v1/funnels", nil, token)
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", tc.role, rec.Body.String())
		list := parseJSONArray(t, rec)
		require.NotEmpty(t, list, "%s must see the account's funnels", tc.role)
		// The webhook secret is an integration credential — admin material.
		for _, raw := range list {
			fn := raw.(map[string]interface{})
			assert.Empty(t, fn["webhook_secret"], "%s must not see the webhook secret", tc.role)
		}
	}
}

func TestVendedorAccess_FunnelCRUDStaysAdminOnly(t *testing.T) {
	f := newRBACFixture(t)
	token := tokenForUser(t, f.accountID, f.vendedorA, "vendedor")

	rec := doRequest("POST", "/v1/funnels", map[string]any{"name": "Novo"}, token)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = doRequest("GET", "/v1/funnels/"+f.funnelID, nil, token)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

func TestVendedorAccess_RepliesToHerOwnLeadFromTheLead(t *testing.T) {
	f := newRBACFixture(t)
	contactID := seedIdentityContact(t, f.accountID, "+5511977770010")
	leadID := seedRBACLead(t, f.accountID, f.funnelID, contactID, &f.vendedorA, "team")
	seedExchange(t, f.accountID, contactID, "5511977770010")

	token := tokenForUser(t, f.accountID, f.vendedorA, "vendedor")
	rec := doRequest("POST", "/v1/leads/"+leadID+"/reply",
		map[string]any{"text": "Ótimo! Mando o orçamento ainda hoje."}, token)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	// The answer is on the same thread, readable where she reads the reply.
	rec = doRequest("GET", "/v1/leads/"+leadID+"/timeline", nil, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out struct {
		Messages []struct {
			Author     string `json:"author"`
			AuthorName string `json:"author_name"`
			Text       string `json:"text"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Messages, 4)
	last := out.Messages[3]
	assert.Equal(t, "Ótimo! Mando o orçamento ainda hoje.", last.Text)
	assert.Equal(t, "team", last.Author)
	assert.Equal(t, "Vendedor", last.AuthorName)

	// And the thread is hers now: the bot is paused so it never answers on top.
	var paused bool
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT is_bot_paused FROM conversations WHERE account_id = $1 AND contact_ref_id = $2`,
		f.accountID, contactID).Scan(&paused))
	assert.True(t, paused)
}

func TestVendedorAccess_ReplyOnALeadNobodyWroteToIs409(t *testing.T) {
	f := newRBACFixture(t)
	token := tokenForUser(t, f.accountID, f.vendedorA, "vendedor")

	rec := doRequest("POST", "/v1/leads/"+f.leadOfA+"/reply",
		map[string]any{"text": "Oi"}, token)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "LEAD_NO_CONVERSATION")
}

func TestVendedorAccess_ReplyOnAColleaguesPrivateLeadIsForbidden(t *testing.T) {
	f := newRBACFixture(t)
	token := tokenForUser(t, f.accountID, f.vendedorA, "vendedor")

	rec := doRequest("POST", "/v1/leads/"+f.privateB+"/reply",
		map[string]any{"text": "Oi"}, token)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

func TestVendedorAccess_EmptyReplyIsRefused(t *testing.T) {
	f := newRBACFixture(t)
	token := tokenForUser(t, f.accountID, f.vendedorA, "vendedor")

	rec := doRequest("POST", "/v1/leads/"+f.leadOfA+"/reply",
		map[string]any{"text": "   "}, token)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "LEAD_REPLY_TEXT_REQUIRED")
}
