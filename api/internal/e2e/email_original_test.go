package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// "View original" end to end: the markup is stored on the way in, a
// short-lived scoped token is minted by an authenticated call, and the render
// endpoint serves it under a policy that blocks what the message might carry.

func mintOriginalToken(t *testing.T, token, messageID string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages/"+messageID+"/original-token", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		return w.Code, ""
	}
	var body struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return w.Code, body.Token
}

func getOriginal(t *testing.T, viewToken string, loadImages bool) *httptest.ResponseRecorder {
	t.Helper()
	url := "/v1/messages/original?token=" + viewToken
	if loadImages {
		url += "&load_images=true"
	}
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
	return w
}

func TestEmailOriginal_StoresAndRendersUnderAPolicy(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	adminToken := tokenForSeededAccount(t, ch.AccountID, "admin")

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "compras@clinica.com.br",
		"html": `<p>Bom dia</p><script>alert(1)</script>` +
			`<img src="https://tracker.example/pixel.gif">`,
	})
	require.Equal(t, http.StatusOK, w.Code)

	msgID := lastMessageID(t, ch.AccountID)
	code, viewToken := mintOriginalToken(t, adminToken, msgID)
	require.Equal(t, http.StatusOK, code)
	require.NotEmpty(t, viewToken)

	res := getOriginal(t, viewToken, false)
	require.Equal(t, http.StatusOK, res.Code)

	body := res.Body.String()
	assert.Contains(t, body, "<p>Bom dia</p>", "the formatting is the whole point")
	assert.NotContains(t, body, "alert(1)", "layer 1: the script is stripped")
	assert.NotContains(t, body, "tracker.example",
		"a remote image is a read receipt — off unless asked")

	// Layer 2: even if the stripper missed something, nothing may execute.
	csp := res.Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "default-src 'none'")
	assert.Contains(t, csp, "sandbox")
	assert.Equal(t, "nosniff", res.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "no-referrer", res.Header().Get("Referrer-Policy"))
	assert.Equal(t, "no-store", res.Header().Get("Cache-Control"))
}

func TestEmailOriginal_LoadsImagesOnlyWhenAsked(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	adminToken := tokenForSeededAccount(t, ch.AccountID, "admin")

	postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "a@b.com",
		// A subject is required for the message to be kept at all: a body
		// that sanitises to nothing AND no subject is not a message.
		"subject": "Proposta",
		"html":    `<img src="https://tracker.example/pixel.gif">`,
	})
	_, viewToken := mintOriginalToken(t, adminToken, lastMessageID(t, ch.AccountID))

	res := getOriginal(t, viewToken, true)
	require.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), "tracker.example")
	assert.Contains(t, res.Header().Get("Content-Security-Policy"),
		"img-src 'self' https: data:")
}

// The route carries no session, so a bad token is the only thing standing
// between a stranger and someone's mail.
func TestEmailOriginal_RefusesAForgedOrAbsentToken(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "a@b.com", "html": "<p>segredo</p>",
	})

	for _, tok := range []string{"", "lixo", "YWJjZGVm"} {
		res := getOriginal(t, tok, false)
		assert.Equal(t, http.StatusNotFound, res.Code, "token %q", tok)
		assert.NotContains(t, res.Body.String(), "segredo")
	}
}

// A message with no stored markup is indistinguishable from one that does
// not exist: the button never appears, and probing gains nothing.
func TestEmailOriginal_PlainTextMessageHasNoOriginal(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	adminToken := tokenForSeededAccount(t, ch.AccountID, "admin")

	postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "a@b.com", "text": "sem html nenhum",
	})

	code, _ := mintOriginalToken(t, adminToken, lastMessageID(t, ch.AccountID))
	assert.Equal(t, http.StatusNotFound, code)
}
