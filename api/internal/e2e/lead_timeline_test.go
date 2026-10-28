package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/auth"
	"github.com/replider/api/internal/core/domain"
)

// The reply to a proposal, where the person who sent it can read it.
//
// A vendedor satisfies neither `admin` nor `attendant`, so every lead route
// answered 403 while the use cases behind them had been written to serve exactly
// that role. The consequence in the product: they sent proposals and could not
// see the answers.
//
// These run over HTTP because the hole was in the ROUTER, not in the use case —
// a test at the use-case layer would have passed the whole time it was broken.

// tokenForUser mints a JWT for a user that already exists, so a test can act as
// a specific seeded vendedor rather than a fresh one.
func tokenForUser(t *testing.T, accountID, userID, role string) string {
	t.Helper()
	token, err := auth.NewJWTService("e2e-test-jwt-secret", 24).GenerateToken(&domain.User{
		ID:        userID,
		AccountID: accountID,
		Name:      "Vendedor",
		Role:      &role,
	})
	require.NoError(t, err)
	return token
}

// seedExchange writes a small conversation with a contact: the approach we sent,
// then the customer's reply.
//
// The two identifiers are deliberately DIFFERENT and both are written, because
// that is the shape production produces: conversations.contact_id is the sender
// as the channel wrote it (rawSender), conversations.contact_ref_id is the FK to
// the contacts row (contactID). The earlier version of this helper put the
// contact id into contact_id and nothing into contact_ref_id — a shape the
// inbound pipeline never writes — so the timeline query joined on contact_id and
// these tests passed for the entire time the feature was broken in production.
// Keeping the two values distinct is what makes them able to fail.
func seedExchange(t *testing.T, accountID, contactID, rawSender string) {
	t.Helper()
	ctx := context.Background()

	channelID := ulid.Make().String()
	_, err := testPool.Exec(ctx,
		`INSERT INTO channels (id, account_id, name, provider, credentials, is_active, webhook_token, created_at, updated_at)
		 VALUES ($1, $2, 'Email', 'email', '{}', true, $3, NOW(), NOW())`,
		channelID, accountID, ulid.Make().String())
	require.NoError(t, err)

	convID := ulid.Make().String()
	_, err = testPool.Exec(ctx,
		`INSERT INTO conversations (id, account_id, channel_id, contact_id, contact_ref_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, 'open', NOW(), NOW())`,
		convID, accountID, channelID, rawSender, contactID)
	require.NoError(t, err)

	base := time.Now().Add(-time.Hour)
	rows := []struct {
		direction, sender, senderName, content string
		at                                     time.Time
	}{
		{"outbound", "01JQXVENDEDORULID000000000", "Rafa", "Segue a proposta que conversamos.", base},
		// messages.sender is the raw channel sender too, never the contact id.
		{"inbound", rawSender, "", "Recebi, vou avaliar com a diretoria.", base.Add(time.Minute)},
		{"outbound", "bot", "", "Posso ajudar com mais alguma coisa?", base.Add(2 * time.Minute)},
	}
	for _, r := range rows {
		_, err = testPool.Exec(ctx,
			`INSERT INTO messages (id, channel_id, conversation_id, sender, sender_name, type, content, direction, created_at)
			 VALUES ($1, $2, $3, $4, NULLIF($5,''), 'text', $6, $7, $8)`,
			ulid.Make().String(), channelID, convID, r.sender, r.senderName, r.content, r.direction, r.at)
		require.NoError(t, err)
	}
}

func TestLeadTimeline_VendedorReadsTheReplyToTheirOwnProposal(t *testing.T) {
	f := newRBACFixture(t)
	contactID := seedIdentityContact(t, f.accountID, "+5511977770001")
	leadID := seedRBACLead(t, f.accountID, f.funnelID, contactID, &f.vendedorA, "team")
	// Bare digits: what Meta actually delivers, and NOT the contact id.
	seedExchange(t, f.accountID, contactID, "5511977770001")

	token := tokenForUser(t, f.accountID, f.vendedorA, "vendedor")
	rec := doRequest("GET", "/v1/leads/"+leadID+"/timeline", nil, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out struct {
		Messages []struct {
			Author     string `json:"author"`
			AuthorName string `json:"author_name"`
			Text       string `json:"text"`
		} `json:"messages"`
		Truncated bool `json:"truncated"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Messages, 3)
	assert.False(t, out.Truncated)

	// Oldest first, so it reads downward like the conversation did.
	assert.Equal(t, "Segue a proposta que conversamos.", out.Messages[0].Text)
	assert.Equal(t, "Recebi, vou avaliar com a diretoria.", out.Messages[1].Text)

	// Who spoke, resolved from an overloaded sender column: inbound is always the
	// customer, 'bot' is the AI, anything else outbound is a teammate.
	assert.Equal(t, "team", out.Messages[0].Author)
	assert.Equal(t, "Rafa", out.Messages[0].AuthorName)
	assert.Equal(t, "customer", out.Messages[1].Author)
	assert.Equal(t, "bot", out.Messages[2].Author)
	// "the bot answered" and "we answered" are different facts about the account.
	assert.Empty(t, out.Messages[2].AuthorName)
}

// A message the channel adapter could not deliver must not read like one that
// reached the customer — the timeline is the vendedor's only view of the
// exchange, and "followed up on a message nobody received" is the failure this
// field exists to prevent.
func TestLeadTimeline_AFailedSendSaysSo(t *testing.T) {
	f := newRBACFixture(t)
	contactID := seedIdentityContact(t, f.accountID, "+5511977770002")
	leadID := seedRBACLead(t, f.accountID, f.funnelID, contactID, &f.vendedorA, "team")
	seedExchange(t, f.accountID, contactID, "5511977770002")

	_, err := testPool.Exec(context.Background(),
		`UPDATE messages SET failed_at = NOW(), failed_reason = 'mailbox unavailable'
		 WHERE content = 'Segue a proposta que conversamos.'`)
	require.NoError(t, err)

	token := tokenForUser(t, f.accountID, f.vendedorA, "vendedor")
	rec := doRequest("GET", "/v1/leads/"+leadID+"/timeline", nil, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out struct {
		Messages []struct {
			Text         string  `json:"text"`
			FailedAt     *string `json:"failed_at"`
			FailedReason string  `json:"failed_reason"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Messages, 3)
	require.NotNil(t, out.Messages[0].FailedAt)
	assert.Equal(t, "mailbox unavailable", out.Messages[0].FailedReason)
	// The delivered ones carry no failure claim.
	assert.Nil(t, out.Messages[1].FailedAt)
	assert.Nil(t, out.Messages[2].FailedAt)
}

// The gate is the LEAD's, not the inbox's: a colleague's private lead answers
// the same way the pipeline already answers it.
func TestLeadTimeline_APrivateLeadOfAColleagueIsNotReadable(t *testing.T) {
	f := newRBACFixture(t)
	contactID := seedIdentityContact(t, f.accountID, "+5511977770002")
	private := seedRBACLead(t, f.accountID, f.funnelID, contactID, &f.vendedorB, "private")
	seedExchange(t, f.accountID, contactID, "5511977770002")

	token := tokenForUser(t, f.accountID, f.vendedorA, "vendedor")
	rec := doRequest("GET", "/v1/leads/"+private+"/timeline", nil, token)

	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "diretoria",
		"a refused read must not leak the conversation it refused")
}

// A prospected lead nobody has written to yet is empty, not an error.
func TestLeadTimeline_NoConversationYetIsEmptyNotAnError(t *testing.T) {
	f := newRBACFixture(t)
	contactID := seedIdentityContact(t, f.accountID, "+5511977770003")
	leadID := seedRBACLead(t, f.accountID, f.funnelID, contactID, &f.vendedorA, "team")

	token := tokenForUser(t, f.accountID, f.vendedorA, "vendedor")
	rec := doRequest("GET", "/v1/leads/"+leadID+"/timeline", nil, token)
	require.Equal(t, http.StatusOK, rec.Code)

	var out struct {
		Messages []json.RawMessage `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Empty(t, out.Messages)
	// An absent list would make the client render "failed to load".
	assert.Contains(t, rec.Body.String(), `"messages":[]`)
}

// The reads a vendedor needs to work a lead at all. Each is actor-scoped; before
// this they all answered 403 and the role was unusable through the API.
func TestLeadTimeline_VendedorCanReachTheLeadReads(t *testing.T) {
	f := newRBACFixture(t)
	token := tokenForUser(t, f.accountID, f.vendedorA, "vendedor")

	for _, path := range []string{
		"/v1/leads/" + f.leadOfA,
		"/v1/leads/overview",
		"/v1/leads/approval-queue",
		"/v1/funnels/" + f.funnelID + "/leads",
		"/v1/funnels/" + f.funnelID + "/board",
	} {
		rec := doRequest("GET", path, nil, token)
		assert.NotEqual(t, http.StatusForbidden, rec.Code,
			"%s must be reachable by a vendedor — the use case behind it is actor-scoped", path)
	}
}

// A colleague's lead is not writable, and the refusal comes BEFORE the payload
// is read — a caller who may not write should learn that, not that their body is
// malformed.
func TestLeadWrites_AColleaguesLeadIsNotWritable(t *testing.T) {
	f := newRBACFixture(t)
	token := tokenForUser(t, f.accountID, f.vendedorA, "vendedor")

	cases := []struct{ method, path string }{
		{"PATCH", "/v1/leads/" + f.privateB + "/metadata"},
		{"PATCH", "/v1/leads/" + f.privateB + "/status"},
		{"PATCH", "/v1/leads/" + f.privateB + "/approach"},
		{"POST", "/v1/leads/" + f.privateB + "/approve-send"},
		{"POST", "/v1/leads/" + f.privateB + "/move"},
	}
	for _, c := range cases {
		// A deliberately malformed body: the answer must still be "forbidden".
		rec := doRequest(c.method, c.path, map[string]any{"nope": 1}, token)
		assert.Equal(t, http.StatusForbidden, rec.Code,
			"%s %s must refuse on ownership before it reads the payload",
			c.method, c.path)
	}
}

// The gap this closes: the pipeline refused an attendant on every READ and
// accepted them on every WRITE. They could not list or open a lead and could
// still change its status, overwrite its draft and approve the send.
func TestLeadWrites_AnAttendantCannotWriteOnTheSalesPipeline(t *testing.T) {
	f := newRBACFixture(t)
	attendant := seedRBACUser(t, f.accountID, string(domain.RoleAtendenteRole))
	token := tokenForUser(t, f.accountID, attendant, "attendant")

	for _, c := range []struct{ method, path string }{
		{"PATCH", "/v1/leads/" + f.unowned + "/status"},
		{"PATCH", "/v1/leads/" + f.unowned + "/approach"},
		{"POST", "/v1/leads/" + f.unowned + "/approve-send"},
	} {
		rec := doRequest(c.method, c.path, map[string]any{"text": "x"}, token)
		assert.Equal(t, http.StatusForbidden, rec.Code,
			"%s %s: an attendant cannot even READ this lead", c.method, c.path)
	}
}

// And the role becomes usable: a vendedor writes on their own lead.
func TestLeadWrites_AVendedorWritesOnTheirOwnLead(t *testing.T) {
	f := newRBACFixture(t)
	token := tokenForUser(t, f.accountID, f.vendedorA, "vendedor")

	rec := doRequest("PATCH", "/v1/leads/"+f.leadOfA+"/approach",
		map[string]any{"text": "Segue a proposta."}, token)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// And on an unowned one — nobody has claimed it, so claiming the work is the
	// point of the role.
	rec = doRequest("PATCH", "/v1/leads/"+f.unowned+"/approach",
		map[string]any{"text": "Primeiro contato."}, token)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
