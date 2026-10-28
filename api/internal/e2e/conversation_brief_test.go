package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The summary answers the question someone actually has when they open a long
// thread after a week away: what was agreed, what is pending, whose move.
//
// No AI provider is configured in the suite, so these pin the parts that must
// be right BEFORE a model is involved — the short-thread guard and the tenant
// scoping. Getting either wrong costs money or leaks a customer's negotiation.

func TestConversationBrief_ShortThreadIsNotWorthAModelCall(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	token := tokenForSeededAccount(t, ch.AccountID, "admin")

	// Two messages: fits on one screen, and a summary would be longer than
	// what it replaces.
	for i, mid := range []string{"<a@x.com>", "<b@x.com>"} {
		require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, map[string]any{
			"from":       "compras@clinica.com.br",
			"subject":    "Proposta",
			"text":       "Mensagem " + string(rune('a'+i)),
			"message_id": mid,
		}).Code)
	}

	convID := lastConversationID(t, ch.AccountID)
	w := doRequest("GET", "/v1/conversations/"+convID+"/summary", nil, token)

	assert.Equal(t, http.StatusNoContent, w.Code,
		"an empty body says 'too short' more honestly than an empty string")
}

// A conversation id from another tenant must read as absent, not as somebody
// else's negotiation.
func TestConversationBrief_AnotherAccountsThreadIsNotFound(t *testing.T) {
	cleanDB(t)
	chA := seedEmailChannel(t)
	chB := seedEmailChannel(t)
	tokenB := tokenForSeededAccount(t, chB.AccountID, "admin")

	require.Equal(t, http.StatusOK, postInboundEmail(t, chA.WebhookToken, map[string]any{
		"from": "compras@clinica.com.br", "subject": "Proposta", "text": "Bom dia",
	}).Code)

	w := doRequest("GET", "/v1/conversations/"+lastConversationID(t, chA.AccountID)+"/summary",
		nil, tokenB)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
