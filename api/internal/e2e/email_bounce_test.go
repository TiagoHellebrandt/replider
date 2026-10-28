package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// End to end for the delivery-failure path. The two things worth proving
// against a real database:
//
//  1. A bounce does not become a conversation. Before this, a DSN parsed as
//     an ordinary message — so MAILER-DAEMON showed up in the inbox as a
//     contact and the agent answered a mail server.
//  2. A hard bounce actually lands in the suppression table, because that
//     row is the only thing standing between the customer and writing to a
//     dead address forever.

func countSuppressions(t *testing.T, accountID string) int {
	t.Helper()
	var n int
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM email_suppressions WHERE account_id = $1`,
		accountID).Scan(&n))
	return n
}

func countConversations(t *testing.T, channelID string) int {
	t.Helper()
	var n int
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM conversations WHERE channel_id = $1`, channelID).Scan(&n))
	return n
}

func TestEmailBounce_HardBounceSuppressesAndCreatesNoConversation(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"type": "email.bounced",
		"data": map[string]any{
			"to": []string{"dono@clinicafechada.com.br"},
			"bounce": map[string]any{
				"type":           "Permanent",
				"diagnosticCode": "smtp; 550 5.1.1 User unknown",
			},
		},
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "bounce_recorded", parseJSON(t, w)["status"])

	assert.Equal(t, 1, countSuppressions(t, ch.AccountID))
	assert.Equal(t, 0, countConversations(t, ch.ID),
		"a bounce is not a message from a person")

	var address, reason, diagnostic string
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT address, reason, diagnostic FROM email_suppressions
		  WHERE account_id = $1`, ch.AccountID).
		Scan(&address, &reason, &diagnostic))
	assert.Equal(t, "dono@clinicafechada.com.br", address)
	assert.Equal(t, "hard_bounce", reason)
	assert.Contains(t, diagnostic, "550 5.1.1")
}

// A full mailbox is emptied. Suppressing here would silently delete a real
// customer from the list, and nobody would ever notice the emails that
// stopped going out.
func TestEmailBounce_SoftBounceSuppressesNothing(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"event-data": map[string]any{
			"event":           "failed",
			"severity":        "temporary",
			"recipient":       "dono@clinica.com.br",
			"delivery-status": map[string]any{"code": 452, "description": "Mailbox full"},
		},
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 0, countSuppressions(t, ch.AccountID))
}

// The one that used to be worst: a DSN parses as an ordinary email, so it
// became a conversation with a robot as the contact and the agent replied
// to it — tokens spent answering a mail server.
func TestEmailBounce_DSNDoesNotBecomeAConversationWithMailerDaemon(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":    "MAILER-DAEMON@provider.com",
		"to":      "comercial@clinica.com.br",
		"subject": "Undelivered Mail Returned to Sender",
		"text": "This is the mail system at host mx.provider.com.\n\n" +
			"Final-Recipient: rfc822; dono@clinicafechada.com.br\n" +
			"Action: failed\n" +
			"Status: 5.1.1\n" +
			"Diagnostic-Code: smtp; 550 5.1.1 User unknown\n",
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 0, countConversations(t, ch.ID))

	// And the address it names — not the daemon — is what got suppressed.
	var address string
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT address FROM email_suppressions WHERE account_id = $1`,
		ch.AccountID).Scan(&address))
	assert.Equal(t, "dono@clinicafechada.com.br", address)
}

// An ordinary reply must still arrive. The bounce check runs first, so a
// false positive here would swallow real customer mail.
func TestEmailBounce_OrdinaryReplyStillBecomesAConversation(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":    "dono@clinica.com.br",
		"to":      "comercial@clinica.com.br",
		"subject": "Re: proposta — a entrega falhou aqui",
		"text":    "Bom dia! O material não chegou, podem reenviar?",
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, countConversations(t, ch.ID))
	assert.Equal(t, 0, countSuppressions(t, ch.AccountID))
}

// Idempotent: providers retry, and a retried bounce must not fail.
func TestEmailBounce_RepeatedBounceIsIdempotent(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	payload := map[string]any{
		"type": "email.bounced",
		"data": map[string]any{
			"to":     []string{"dono@clinicafechada.com.br"},
			"bounce": map[string]any{"type": "Permanent"},
		},
	}
	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, payload).Code)
	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, payload).Code)

	assert.Equal(t, 1, countSuppressions(t, ch.AccountID))
}

// Case-insensitive by index: Dono@Clinica.com and dono@clinica.com are the
// same mailbox, and suppressing one while still writing to the other would
// make the whole list useless.
func TestEmailBounce_SuppressionIsCaseInsensitive(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	for _, addr := range []string{"Dono@ClinicaFechada.com.br", "dono@clinicafechada.com.br"} {
		require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, map[string]any{
			"type": "email.bounced",
			"data": map[string]any{
				"to":     []string{addr},
				"bounce": map[string]any{"type": "Permanent"},
			},
		}).Code)
	}

	assert.Equal(t, 1, countSuppressions(t, ch.AccountID))
}

// ── The list a person can actually see and undo ───────────────────────────

// A silent block teaches a salesperson to distrust the whole tool: their
// message simply never goes out, with no list to consult.
func TestEmailSuppression_IsVisibleAndReleasable(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	token := tokenForSeededAccount(t, ch.AccountID, "admin")

	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, map[string]any{
		"type": "email.bounced",
		"data": map[string]any{
			"to":     []string{"dono@clinicafechada.com.br"},
			"bounce": map[string]any{"type": "Permanent", "diagnosticCode": "550 5.1.1 User unknown"},
		},
	}).Code)

	list := parseJSONArray(t, doRequest("GET", "/v1/email-suppressions", nil, token))
	require.Len(t, list, 1)
	row := list[0].(map[string]any)
	assert.Equal(t, "dono@clinicafechada.com.br", row["address"])
	assert.Equal(t, "hard_bounce", row["reason"])
	// The remote server's own words: the answer to "why can't I email them?".
	assert.Contains(t, row["diagnostic"], "550 5.1.1")

	// A suppression can be wrong — an address is fixed, a mailbox emptied.
	w := doRequest("DELETE", "/v1/email-suppressions/dono@clinicafechada.com.br", nil, token)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.Equal(t, 0, countSuppressions(t, ch.AccountID))
}

// Releasing an address that is not on the list satisfies the caller's intent
// either way; a 404 would only invite a retry loop.
func TestEmailSuppression_ReleaseIsIdempotent(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	token := tokenForSeededAccount(t, ch.AccountID, "admin")

	w := doRequest("DELETE", "/v1/email-suppressions/ninguem@lugar.nenhum", nil, token)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// One account's dead addresses are not another's.
func TestEmailSuppression_IsScopedToTheAccount(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, map[string]any{
		"type": "email.bounced",
		"data": map[string]any{
			"to":     []string{"dono@clinicafechada.com.br"},
			"bounce": map[string]any{"type": "Permanent"},
		},
	}).Code)

	otherAccount := seedIdentityAccount(t)
	otherToken := tokenForSeededAccount(t, otherAccount, "admin")

	list := parseJSONArray(t, doRequest("GET", "/v1/email-suppressions", nil, otherToken))
	assert.Empty(t, list)
}
