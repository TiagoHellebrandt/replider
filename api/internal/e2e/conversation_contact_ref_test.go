package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
)

// conversations.contact_ref_id: the column that says WHO a thread is with.
//
// It existed and was indexed from migration 000009 and nothing ever wrote it.
// Three readers had been written against it anyway — the lead panel's
// "open the conversation" lookup, the lead timeline, and the LGPD opt-out guard
// on auto-resume — so all three were joining on NULL. In the product: the
// vendedora sent a proposal, the customer replied, and the lead showed nothing.
//
// Everything below drives the REAL inbound path. Seeding the conversation row
// directly is what let this survive: the old fixture wrote the contact ULID into
// contact_id, a shape the pipeline never produces, and passed while production
// was broken.

// seedFunnelOn creates a funnel on an EXISTING account, unlike newRBACFixture
// which owns its own account. The webhook needs the channel and the lead needs
// the funnel to be on the same account as that channel.
func seedFunnelOn(t *testing.T, accountID string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO sales_funnels (id, account_id, name, created_at, updated_at)
		 VALUES ($1, $2, 'Funil contact_ref', NOW(), NOW())`, id, accountID)
	require.NoError(t, err)
	return id
}

// contactRefOf reads back the two identifiers the pipeline wrote for a channel's
// only conversation: the conversation id, the raw sender, and the contact FK.
func contactRefOf(t *testing.T, channelID string) (convID, rawSender string, ref *string) {
	t.Helper()
	err := testPool.QueryRow(context.Background(),
		`SELECT id, contact_id, contact_ref_id FROM conversations WHERE channel_id = $1`,
		channelID).Scan(&convID, &rawSender, &ref)
	require.NoError(t, err)
	return convID, rawSender, ref
}

// The write itself: an inbound message must leave the thread pointing at a
// contacts row, while contact_id keeps the sender verbatim because routing the
// NEXT inbound needs it in the channel's own spelling.
func TestInboundWebhook_BindsTheConversationToTheContactRow(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	ch := seedEmailChannel(t)

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":       "Compras <Compras@Hospital.com.BR>",
		"to":         "comercial@clinica.com.br",
		"subject":    "Re: Orçamento",
		"text":       "Recebi, vou avaliar com a diretoria.",
		"message_id": "<ref1@hospital.com.br>",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	_, rawSender, ref := contactRefOf(t, ch.ID)
	assert.Equal(t, "compras@hospital.com.br", rawSender,
		"contact_id stays the sender as the channel wrote it")
	require.NotNil(t, ref, "contact_ref_id must be stamped at creation")

	// And it points at the contact the identity resolver actually registered,
	// not at some other row that merely happens to exist.
	var contactID string
	err := testPool.QueryRow(ctx,
		`SELECT ci.contact_id FROM contact_identities ci
		  WHERE ci.account_id = $1 AND ci.kind = 'email' AND ci.value = $2`,
		ch.AccountID, "compras@hospital.com.br").Scan(&contactID)
	require.NoError(t, err)
	assert.Equal(t, contactID, *ref)

	// The two columns must not hold the same value — if they did, every join
	// below would pass for the wrong reason.
	assert.NotEqual(t, rawSender, *ref)
}

// The break the audit found, end to end: the reply arrives through the webhook
// and the vendedora reads it on the lead. Nothing here seeds a conversation.
func TestLeadTimeline_ShowsAReplyThatArrivedThroughTheWebhook(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	accountID := ch.AccountID

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":       "Compras <compras@hospital.com.br>",
		"to":         "comercial@clinica.com.br",
		"subject":    "Re: Orçamento de equipamentos",
		"text":       "Recebi, vou avaliar com a diretoria.",
		"message_id": "<ref2@hospital.com.br>",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	_, _, ref := contactRefOf(t, ch.ID)
	require.NotNil(t, ref)

	// A lead on that same contact — which is all a lead ever knows about the
	// person. It has no idea which channel reached them.
	funnelID := seedFunnelOn(t, accountID)
	vendedor := seedRBACUser(t, accountID, string(domain.RoleVendedor))
	leadID := seedRBACLead(t, accountID, funnelID, *ref, &vendedor, "team")

	token := tokenForUser(t, accountID, vendedor, string(domain.RoleVendedor))
	rec := doRequest("GET", "/v1/leads/"+leadID+"/timeline", nil, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out struct {
		Messages []struct {
			Author string `json:"author"`
			Text   string `json:"text"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Messages, 1,
		"the reply the customer sent must be on the lead — this was empty in production")
	assert.Equal(t, "Recebi, vou avaliar com a diretoria.", out.Messages[0].Text)
	assert.Equal(t, "customer", out.Messages[0].Author)

	// The same fact through the other reader: "open the conversation" from the
	// lead panel resolves the thread by contact, not by channel.
	conv, err := postgres.NewConversationRepository(testPool).
		FindByAccountContact(context.Background(), accountID, *ref)
	require.NoError(t, err)
	require.NotNil(t, conv, "the lead panel's conversation link must resolve")
}

// The side effect that matters most, because it is about consent rather than
// about a screen.
//
// AutoResumeStuckConversations un-pauses threads an attendant abandoned, and is
// supposed to skip anyone who opted out via
//
//	NOT EXISTS (SELECT 1 FROM contacts ct
//	             WHERE ct.id = c.contact_ref_id AND ct.opted_out_at IS NOT NULL)
//
// With contact_ref_id always NULL that subquery matched nothing, so the NOT
// EXISTS was vacuously true and the guard was dead: the bot could start talking
// again to someone who had asked it to stop. The control thread in the same test
// is what makes the assertion mean anything — without it, a query that resumes
// nothing at all would look like a working guard.
func TestAutoResume_SkipsAThreadWhoseContactOptedOut(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	quiet := seedEmailChannel(t)
	quietConv, quietRef := inboundThenPause(t, quiet, "quieto@hospital.com.br", "<optout@x>")
	// The control thread is a CHAT: auto-resume skips email channels
	// altogether (the AI never answers email, so a paused email thread is a
	// human who assumed it on purpose — see conversation_reopen_test.go).
	activeConv := telegramInboundThenPause(t)

	// Only one of them asked us to stop.
	_, err := testPool.Exec(ctx,
		`UPDATE contacts SET opted_out_at = NOW() WHERE id = $1`, quietRef)
	require.NoError(t, err)

	resumed, err := postgres.NewConversationRepository(testPool).
		AutoResumeStuckConversations(ctx, 30)
	require.NoError(t, err)
	assert.EqualValues(t, 1, resumed,
		"one thread must resume, or this test would pass on a query that resumes nothing")

	assert.True(t, isBotPaused(t, quietConv),
		"a contact who opted out must stay paused — resuming them is the LGPD breach")
	assert.False(t, isBotPaused(t, activeConv))
}

// inboundThenPause drives a real inbound on the channel, then puts the thread in
// the state the resume worker looks for: bot-paused and idle past the window.
// Returns the conversation id and the contact it resolved to.
func inboundThenPause(t *testing.T, ch *domain.Channel, from, messageID string) (convID, contactID string) {
	t.Helper()
	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":       from,
		"to":         "comercial@clinica.com.br",
		"subject":    "Orçamento",
		"text":       "Alguém pode me atender?",
		"message_id": messageID,
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	convID, _, ref := contactRefOf(t, ch.ID)
	require.NotNil(t, ref, "the resume guard reads contact_ref_id — a NULL here is the bug")
	_, err := testPool.Exec(context.Background(),
		`UPDATE conversations
		    SET is_bot_paused = TRUE, status = 'pending_human',
		        updated_at = NOW() - INTERVAL '2 hours'
		  WHERE id = $1`, convID)
	require.NoError(t, err)
	return convID, *ref
}

// The backfill, tested as SQL rather than as a migration run.
//
// TestMain drops and re-applies every migration against an empty database, so
// 000196 always executes with nothing to repair — passing there proves only that
// the statements parse. This re-runs the file's own SQL over rows in the two
// legacy shapes it exists for, which is the only way to know it matches anything.
func TestContactRefBackfill_RepairsBothLegacyShapes(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)

	channelID := ulid.Make().String()
	_, err := testPool.Exec(ctx,
		`INSERT INTO channels (id, account_id, name, provider, credentials, is_active, webhook_token, created_at, updated_at)
		 VALUES ($1, $2, 'Legado', 'whatsapp', '{}', true, $3, NOW(), NOW())`,
		channelID, accountID, ulid.Make().String())
	require.NoError(t, err)

	// Shape 1 — recognisable by contacts.external_id, which holds the sender
	// verbatim. Unique per account, so this match is unambiguous.
	byExternal := ulid.Make().String()
	_, err = testPool.Exec(ctx,
		`INSERT INTO contacts (id, account_id, external_id, created_at, updated_at, first_seen_at, last_seen_at)
		 VALUES ($1, $2, '5565988880001', NOW(), NOW(), NOW(), NOW())`, byExternal, accountID)
	require.NoError(t, err)

	// Shape 2 — only the identity index knows this one, and it knows it in the
	// NORMALISED form. Meta delivered bare digits; the identity is E.164. A
	// verbatim comparison misses it, which is why there are two passes.
	byIdentity := ulid.Make().String()
	_, err = testPool.Exec(ctx,
		`INSERT INTO contacts (id, account_id, external_id, created_at, updated_at, first_seen_at, last_seen_at)
		 VALUES ($1, $2, $3, NOW(), NOW(), NOW(), NOW())`,
		byIdentity, accountID, "contato-sem-external-"+byIdentity[20:])
	require.NoError(t, err)
	_, err = testPool.Exec(ctx,
		`INSERT INTO contact_identities (id, account_id, contact_id, kind, value, created_at, updated_at)
		 VALUES ($1, $2, $3, 'phone', '+5565988880002', NOW(), NOW())`,
		ulid.Make().String(), accountID, byIdentity)
	require.NoError(t, err)

	// Three threads: one per shape, plus one from a sender nobody ever indexed.
	shapeOne := seedLegacyConversation(t, accountID, channelID, "5565988880001")
	shapeTwo := seedLegacyConversation(t, accountID, channelID, "5565988880002")
	unknown := seedLegacyConversation(t, accountID, channelID, "5565988889999")

	sql, err := os.ReadFile(
		findMigrationsPath() + "/000196_conversations_contact_ref_backfill.up.sql")
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, string(sql))
	require.NoError(t, err)

	assert.Equal(t, byExternal, refOf(t, shapeOne))
	assert.Equal(t, byIdentity, refOf(t, shapeTwo),
		"the normalised form must be matched too, or every WhatsApp thread stays orphaned")
	// Deliberately still NULL. Attaching this thread to a guessed contact would
	// put one customer's messages on another customer's lead, which is worse
	// than an empty timeline.
	assert.Empty(t, refOf(t, unknown))
}

func seedLegacyConversation(t *testing.T, accountID, channelID, rawSender string) string {
	t.Helper()
	id := ulid.Make().String()
	// contact_ref_id left out entirely — the pre-000196 shape.
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO conversations (id, account_id, channel_id, contact_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'open', NOW(), NOW())`,
		id, accountID, channelID, rawSender)
	require.NoError(t, err)
	return id
}

func refOf(t *testing.T, convID string) string {
	t.Helper()
	var ref *string
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT contact_ref_id FROM conversations WHERE id = $1`, convID).Scan(&ref))
	if ref == nil {
		return ""
	}
	return *ref
}

func isBotPaused(t *testing.T, convID string) bool {
	t.Helper()
	var paused bool
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT is_bot_paused FROM conversations WHERE id = $1`, convID).Scan(&paused))
	return paused
}
