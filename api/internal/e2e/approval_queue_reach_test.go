package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A door is not an address, and the approval queue has to be able to say so.
//
// Production, 2026-09-10: an account with a connected mailbox, 70 contacts,
// 52 of them carrying a phone and NOT ONE an e-mail, and 50 finished offers
// waiting in the queue. The account-level "you have no channel" banner stayed
// silent — correctly, the account did have a mailbox — and every card offered
// an enabled approve button for a letter that could never be posted.
//
// Driven through the real router and the real DB because that is the only way
// to catch the failure this actually was: two reads (channels, contacts) whose
// wiring is optional, where an unwired reader answers the permissive way and
// the screen goes quiet.
func TestApprovalQueue_ACardWithNoAddressSaysSo(t *testing.T) {
	f := newRBACFixture(t)
	ctx := context.Background()

	// The account CAN send: a live mailbox.
	_, err := testPool.Exec(ctx,
		`INSERT INTO channels (id, account_id, provider, name, webhook_token, credentials, is_active, created_at, updated_at)
		 VALUES ($1, $2, 'email', 'Caixa comercial', $3, '{}'::jsonb, TRUE, NOW(), NOW())`,
		ulid.Make().String(), f.accountID, ulid.Make().String())
	require.NoError(t, err)

	// One recipient it can reach, one it cannot.
	reachable := seedContactWithAddress(t, f.accountID, "contato@bemviver.com.br", "")
	unreachable := seedContactWithAddress(t, f.accountID, "", "+5565988887777")

	reachableLead := seedQueuedLead(t, f.accountID, f.funnelID, reachable, &f.vendedorA)
	unreachableLead := seedQueuedLead(t, f.accountID, f.funnelID, unreachable, &f.vendedorA)

	token := tokenForUser(t, f.accountID, f.vendedorA, "vendedor")
	rec := doRequest("GET", "/v1/leads/approval-queue", nil, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out struct {
		Items []struct {
			ID                string `json:"id"`
			SendBlockedReason string `json:"send_blocked_reason"`
		} `json:"items"`
		SendBlockedReason string `json:"send_blocked_reason"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Items, 2)

	byID := map[string]string{}
	for _, item := range out.Items {
		byID[item.ID] = item.SendBlockedReason
	}
	assert.Equal(t, "NO_ADDRESS", byID[unreachableLead],
		"a phone-only contact is out of reach of a mailbox-only account")
	assert.Empty(t, byID[reachableLead],
		"the card beside it is perfectly sendable and must not be flagged")

	// And the account-level banner stays silent: the account really can send.
	// Blocking the whole screen would tell someone with sendable offers that
	// nothing can leave.
	assert.Empty(t, out.SendBlockedReason)
}

// The same phone-only recipient, with WhatsApp connected, is reachable. The
// flag is the CROSSING of address and channel — never either one alone, or it
// would condemn the accounts this product is built for.
func TestApprovalQueue_APhoneIsAnAddressWhenWhatsAppIsConnected(t *testing.T) {
	f := newRBACFixture(t)
	ctx := context.Background()

	_, err := testPool.Exec(ctx,
		`INSERT INTO channels (id, account_id, provider, name, webhook_token, credentials, is_active, created_at, updated_at)
		 VALUES ($1, $2, 'whatsapp', 'WhatsApp comercial', $3, '{}'::jsonb, TRUE, NOW(), NOW())`,
		ulid.Make().String(), f.accountID, ulid.Make().String())
	require.NoError(t, err)

	contactID := seedContactWithAddress(t, f.accountID, "", "+5565988887777")
	leadID := seedQueuedLead(t, f.accountID, f.funnelID, contactID, &f.vendedorA)

	token := tokenForUser(t, f.accountID, f.vendedorA, "vendedor")
	rec := doRequest("GET", "/v1/leads/approval-queue", nil, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out struct {
		Items []struct {
			ID                string `json:"id"`
			SendBlockedReason string `json:"send_blocked_reason"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Items, 1)
	assert.Equal(t, leadID, out.Items[0].ID)
	assert.Empty(t, out.Items[0].SendBlockedReason)
}

func seedContactWithAddress(t *testing.T, accountID, email, phone string) string {
	t.Helper()
	id := ulid.Make().String()
	var emailArg, phoneArg *string
	if email != "" {
		emailArg = &email
	}
	if phone != "" {
		phoneArg = &phone
	}
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO contacts (id, account_id, external_id, email, phone,
		                       created_at, updated_at, first_seen_at, last_seen_at)
		 VALUES ($1, $2, $1, $3, $4, NOW(), NOW(), NOW(), NOW())`,
		id, accountID, emailArg, phoneArg)
	require.NoError(t, err)
	return id
}

func seedQueuedLead(t *testing.T, accountID, funnelID, contactID string, owner *string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO leads (id, account_id, funnel_id, contact_id, status, approach_draft,
		                    owner_user_id, visibility, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'pending_approval', 'Olá! Sobre o pregão…', $5, 'team', NOW(), NOW())`,
		id, accountID, funnelID, contactID, owner)
	require.NoError(t, err)
	return id
}
