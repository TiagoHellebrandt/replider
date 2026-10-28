package e2e

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The invite door beyond the journey's happy path: every way a token can be
// bad answers the SAME refusal, revocation works, and the owner sees the
// unfiltered plan catalogue the invites point at.

func ownerTokenForInvites(t *testing.T) string {
	t.Helper()
	registerUser(t, "Replider Platform", "owner@replider.com", "senha-forte-123")
	return login(t, "owner@replider.com", "senha-forte-123")
}

func emitInvite(t *testing.T, ownerToken, email string) (secret, id string) {
	t.Helper()
	w := doRequest("POST", "/v1/owner/invites", map[string]any{
		"email":   email,
		"plan_id": "01PLAN_ENTERPRISE_00000",
	}, ownerToken)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var out struct {
		Token  string `json:"token"`
		Invite struct {
			ID string `json:"id"`
		} `json:"invite"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out.Token, out.Invite.ID
}

func redeem(t *testing.T, email, token string) *string {
	t.Helper()
	w := doRequest("POST", "/v1/accounts", map[string]any{
		"account_name": "Cliente", "admin_name": "Cliente",
		"admin_email": email, "password": "senha-forte-123",
		"terms_accepted": true, "invite_token": token,
	}, "")
	body := w.Body.String()
	if w.Code == http.StatusCreated {
		return nil
	}
	return &body
}

// Every bad token — wrong e-mail, revoked, garbage — must be told apart by
// NOTHING: same status, same code, same message. Distinguishing them is an
// enumeration oracle.
func TestInvites_EveryRefusalIsIndistinguishable(t *testing.T) {
	cleanDB(t)
	ownerToken := ownerTokenForInvites(t)

	secret, id := emitInvite(t, ownerToken, "certa@clinica.com.br")

	wrongEmail := redeem(t, "errada@clinica.com.br", secret)
	require.NotNil(t, wrongEmail, "e-mail fora do binding não passa")

	// Revoke, then try the right e-mail: revoked must read exactly like
	// wrong-email did.
	w := doRequest("POST", "/v1/owner/invites/"+id+"/revoke", nil, ownerToken)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	revoked := redeem(t, "certa@clinica.com.br", secret)
	require.NotNil(t, revoked)

	garbage := redeem(t, "certa@clinica.com.br", "deadbeef")
	require.NotNil(t, garbage)

	assert.Equal(t, *wrongEmail, *revoked, "recusas idênticas — sem oráculo")
	assert.Equal(t, *revoked, *garbage)
	assert.Contains(t, *garbage, "INVITE_INVALID")

	// The probe answers the same uniform "no" for all of them.
	for _, tok := range []string{secret, "deadbeef", ""} {
		w := doRequest("GET", "/v1/invites/probe?token="+tok, nil, "")
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"valid":false`)
	}
}

func TestInvites_OwnerListsPendingAndRevokedDisappears(t *testing.T) {
	cleanDB(t)
	ownerToken := ownerTokenForInvites(t)
	_, id := emitInvite(t, ownerToken, "pendente@clinica.com.br")

	w := doRequest("GET", "/v1/owner/invites", nil, ownerToken)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "pendente@clinica.com.br")
	assert.NotContains(t, w.Body.String(), "token_hash",
		"a listagem nunca expõe hash nenhum")

	w = doRequest("POST", "/v1/owner/invites/"+id+"/revoke", nil, ownerToken)
	require.Equal(t, http.StatusNoContent, w.Code)
	// Revoking twice is a conflict, not a silent success.
	w = doRequest("POST", "/v1/owner/invites/"+id+"/revoke", nil, ownerToken)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())

	w = doRequest("GET", "/v1/owner/invites", nil, ownerToken)
	assert.NotContains(t, w.Body.String(), "pendente@clinica.com.br")
}

// After the catalogue collapsed to a single Enterprise contract tier
// (migration 000228), the owner catalogue must list Enterprise — that is what
// an invite points at — and the retired SDR tiers must be gone from both
// surfaces.
func TestInvites_OwnerPlansListsTheContractTier(t *testing.T) {
	cleanDB(t)
	ownerToken := ownerTokenForInvites(t)

	w := doRequest("GET", "/v1/owner/plans", nil, ownerToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "01PLAN_ENTERPRISE_00000")
	assert.NotContains(t, w.Body.String(), "01PLAN_SDR_VENDEDOR_000",
		"the retired SDR tiers are deleted, not merely hidden")
}

// The owner surface is super-admin only — an ordinary admin gets nothing.
func TestInvites_OrdinaryAdminCannotEmit(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Conta Comum", "comum@empresa.com.br", "senha-forte-123")
	token := login(t, "comum@empresa.com.br", "senha-forte-123")

	w := doRequest("POST", "/v1/owner/invites", map[string]any{
		"email": "x@y.com.br", "plan_id": "01PLAN_ENTERPRISE_00000",
	}, token)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}
