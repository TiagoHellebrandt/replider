package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
)

// Inbox facts the 2026-09-01 analysis found broken, each driven end to end:
//
//   - "Resolver" closed the thread and the customer's next reply opened a
//     SECOND conversation with no history (FindByContact skips `closed`).
//   - The lead-stage chip was blank for every lead: the LATERAL join compared
//     the lead's contact ULID with the raw channel sender.
//   - "Unread" was faked from status = pending_human; messages.read_at was
//     written by POST /read and read by nobody.
//   - Auto-resume un-paused email threads a human had deliberately assumed.

func conversationState(t *testing.T, convID string) (status string, resolvedAt *time.Time, waitingSince *time.Time) {
	t.Helper()
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT status, resolved_at, waiting_since FROM conversations WHERE id = $1`, convID).
		Scan(&status, &resolvedAt, &waitingSince))
	return
}

func conversationsOnChannel(t *testing.T, channelID string) int {
	t.Helper()
	var n int
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM conversations WHERE channel_id = $1`, channelID).Scan(&n))
	return n
}

func TestResolvedConversation_ReopensWhenTheCustomerAnswers(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	token := tokenForSeededAccount(t, ch.AccountID, "admin")

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "Compras <compras@hospital.com.br>", "to": "comercial@clinica.com.br",
		"subject": "Orçamento", "text": "Pode mandar a proposta?", "message_id": "<reopen-1@x>",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	convID, _, _ := contactRefOf(t, ch.ID)

	w = doRequest("PATCH", "/v1/conversations/"+convID+"/resolve", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	status, resolvedAt, _ := conversationState(t, convID)
	require.Equal(t, "closed", status)
	require.NotNil(t, resolvedAt)

	// The customer writes again, days later, on the same thread.
	w = postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "Compras <compras@hospital.com.br>", "to": "comercial@clinica.com.br",
		"subject": "Re: Orçamento", "text": "Voltei a pensar nisso.", "message_id": "<reopen-2@x>",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Equal(t, 1, conversationsOnChannel(t, ch.ID),
		"the reply must land on the resolved thread, not open a second conversation without the history")
	status, resolvedAt, waitingSince := conversationState(t, convID)
	assert.Equal(t, "open", status)
	assert.Nil(t, resolvedAt)
	assert.NotNil(t, waitingSince, "a reopened thread is waiting on a human again")

	var messages int
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM messages WHERE conversation_id = $1`, convID).Scan(&messages))
	assert.Equal(t, 2, messages, "both messages on the one thread — that IS the history")
}

func TestReopenEndpoint_FlipsAResolvedThreadBack(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	token := tokenForSeededAccount(t, ch.AccountID, "admin")
	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "a@hospital.com.br", "to": "comercial@clinica.com.br",
		"subject": "Oi", "text": "Olá", "message_id": "<reopen-ep-1@x>",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	convID, _, _ := contactRefOf(t, ch.ID)

	// Reopening an open thread is a no-op 200, not an error.
	w = doRequest("PATCH", "/v1/conversations/"+convID+"/reopen", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = doRequest("PATCH", "/v1/conversations/"+convID+"/resolve", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = doRequest("PATCH", "/v1/conversations/"+convID+"/reopen", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "open", parseJSON(t, w)["status"])
	status, resolvedAt, _ := conversationState(t, convID)
	assert.Equal(t, "open", status)
	assert.Nil(t, resolvedAt)

	// Another account cannot reopen it.
	other := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")
	w = doRequest("PATCH", "/v1/conversations/"+convID+"/reopen", nil, other)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUnreadCount_FollowsMessagesReadAt(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	token := tokenForSeededAccount(t, ch.AccountID, "admin")
	for _, id := range []string{"<u1@x>", "<u2@x>"} {
		w := postInboundEmail(t, ch.WebhookToken, map[string]any{
			"from": "b@hospital.com.br", "to": "comercial@clinica.com.br",
			"subject": "Oi", "text": "Mensagem " + id, "message_id": id,
		})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	convID, _, _ := contactRefOf(t, ch.ID)

	w := doRequest("GET", "/v1/conversations/unread-count", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := parseJSON(t, w)
	assert.EqualValues(t, 1, body["count"], "one thread with unread customer messages")
	assert.EqualValues(t, 2, body["messages"])

	// The list row carries the same number.
	w = doRequest("GET", "/v1/conversations?status=open", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	rows := parseJSONArray(t, w)
	require.Len(t, rows, 1)
	assert.EqualValues(t, 2, rows[0].(map[string]interface{})["unread_count"])

	// Opening the thread clears it — on the row and on the badge.
	w = doRequest("POST", "/v1/conversations/"+convID+"/read", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = doRequest("GET", "/v1/conversations/unread-count", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	assert.EqualValues(t, 0, parseJSON(t, w)["count"])
	w = doRequest("GET", "/v1/conversations?status=open", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	assert.EqualValues(t, 0, parseJSONArray(t, w)[0].(map[string]interface{})["unread_count"])

	// A resolved thread's leftovers never count.
	w = postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "b@hospital.com.br", "to": "comercial@clinica.com.br",
		"subject": "Oi", "text": "Mais uma", "message_id": "<u3@x>",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = doRequest("PATCH", "/v1/conversations/"+convID+"/resolve", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	w = doRequest("GET", "/v1/conversations/unread-count", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	assert.EqualValues(t, 0, parseJSON(t, w)["count"])
}

// The inbox chip: the thread came in through the webhook, the lead was bound
// to the CONTACT (the only thing a lead knows), and the list must show it.
func TestInboxList_ShowsTheLeadStageThroughContactRef(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	ch := seedEmailChannel(t)
	token := tokenForSeededAccount(t, ch.AccountID, "admin")
	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "Compras <compras@hospital.com.br>", "to": "comercial@clinica.com.br",
		"subject": "Re: Proposta", "text": "Recebi.", "message_id": "<stage-1@x>",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	_, rawSender, ref := contactRefOf(t, ch.ID)
	require.NotNil(t, ref)
	require.NotEqual(t, rawSender, *ref, "the two columns must differ or the join passes for the wrong reason")

	funnelID := seedFunnelOn(t, ch.AccountID)
	leadID := seedRBACLead(t, ch.AccountID, funnelID, *ref, nil, "team")
	stageID := ulid.Make().String()
	_, err := testPool.Exec(ctx,
		`INSERT INTO funnel_stages (id, account_id, funnel_id, name, position, maps_to_status, created_at, updated_at)
		 VALUES ($1, $2, $3, 'Negociação', 2, 'negotiating', NOW(), NOW())`, stageID, ch.AccountID, funnelID)
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, `UPDATE leads SET stage_id = $2 WHERE id = $1`, leadID, stageID)
	require.NoError(t, err)

	for _, sort := range []string{"", "importance", "first_in_conv", "oldest"} {
		w = doRequest("GET", "/v1/conversations?status=open&sort="+sort, nil, token)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		rows := parseJSONArray(t, w)
		require.Len(t, rows, 1, "sort=%q", sort)
		row := rows[0].(map[string]interface{})
		assert.Equal(t, leadID, row["lead_id"], "sort=%q", sort)
		assert.Equal(t, "Negociação", row["lead_stage_name"], "sort=%q", sort)
	}
}

// Search and priority are SQL filters: a match past the page limit is found.
func TestInboxList_SearchRunsBeforePagination(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	token := tokenForSeededAccount(t, ch.AccountID, "admin")
	for i, from := range []string{"ana@a.com", "bia@b.com", "carla@c.com"} {
		w := postInboundEmail(t, ch.WebhookToken, map[string]any{
			"from": from, "to": "comercial@clinica.com.br", "subject": "Oi",
			"text": "Mensagem número " + string(rune('0'+i)), "message_id": "<q" + string(rune('0'+i)) + "@x>",
		})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		time.Sleep(5 * time.Millisecond) // distinct last_customer_message_at ordering
	}
	// Ana wrote first, so she ranks LAST by recency; with limit=1 a Go-side
	// filter over the page would never see her.
	w := doRequest("GET", "/v1/conversations?status=open&q=ana&limit=1", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	rows := parseJSONArray(t, w)
	require.Len(t, rows, 1)
	assert.Equal(t, "ana@a.com", rows[0].(map[string]interface{})["contact_id"])

	// LIKE metacharacters are literal.
	w = doRequest("GET", "/v1/conversations?status=open&q=%25", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Len(t, parseJSONArray(t, w), 0, "a literal %% matches nobody")

	// Priority too.
	var convID string
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT id FROM conversations WHERE channel_id = $1 AND contact_id = 'bia@b.com'`, ch.ID).Scan(&convID))
	_, err := testPool.Exec(context.Background(), `UPDATE conversations SET priority = 'urgent' WHERE id = $1`, convID)
	require.NoError(t, err)
	w = doRequest("GET", "/v1/conversations?status=open&priority=urgent&limit=1", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	rows = parseJSONArray(t, w)
	require.Len(t, rows, 1)
	assert.Equal(t, convID, rows[0].(map[string]interface{})["id"])
}

// Transcript load-more: the page BEFORE a cursor, oldest first.
func TestMessages_BeforeCursorPagesOlderMessages(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	token := tokenForSeededAccount(t, ch.AccountID, "admin")
	for i := 0; i < 5; i++ {
		w := postInboundEmail(t, ch.WebhookToken, map[string]any{
			"from": "d@d.com", "to": "comercial@clinica.com.br", "subject": "Oi",
			"text": "m" + string(rune('0'+i)), "message_id": "<m" + string(rune('0'+i)) + "@x>",
		})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		time.Sleep(5 * time.Millisecond)
	}
	convID, _, _ := contactRefOf(t, ch.ID)

	w := doRequest("GET", "/v1/conversations/"+convID+"/messages?limit=2&offset=3", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	newest := parseJSONArray(t, w)
	require.Len(t, newest, 2)
	oldestHeld := newest[0].(map[string]interface{})
	assert.Equal(t, "m3", oldestHeld["content"])

	w = doRequest("GET", "/v1/conversations/"+convID+"/messages?limit=2&before="+oldestHeld["created_at"].(string), nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	older := parseJSONArray(t, w)
	require.Len(t, older, 2)
	assert.Equal(t, "m1", older[0].(map[string]interface{})["content"], "chronological within the page")
	assert.Equal(t, "m2", older[1].(map[string]interface{})["content"])

	w = doRequest("GET", "/v1/conversations/"+convID+"/messages?before=not-a-time", nil, token)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// The AI never answers email, so a paused email thread is a human who took
// it on purpose. Auto-resume must leave it alone — and still resume the
// abandoned chat thread next to it, or this passes on a query that resumes
// nothing.
func TestAutoResume_LeavesEmailThreadsPaused(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	email := seedEmailChannel(t)
	emailConv, _ := inboundThenPause(t, email, "humano@hospital.com.br", "<email-paused@x>")
	chatConv := telegramInboundThenPause(t)

	resumed, err := postgres.NewConversationRepository(testPool).AutoResumeStuckConversations(ctx, 30)
	require.NoError(t, err)
	assert.EqualValues(t, 1, resumed)
	assert.True(t, isBotPaused(t, emailConv), "an assumed email thread must stay with the human")
	assert.False(t, isBotPaused(t, chatConv), "the abandoned chat thread resumes as before")
}

// telegramInboundThenPause drives a real Telegram inbound on a fresh
// channel and leaves the thread bot-paused and idle past the window.
func telegramInboundThenPause(t *testing.T) string {
	t.Helper()
	ch := seedResolverChannel(t, "telegram")
	update := map[string]any{
		"update_id": 1,
		"message": map[string]any{
			"message_id": 1,
			"from":       map[string]any{"id": 4242, "first_name": "Ana"},
			"chat":       map[string]any{"id": 4242, "type": "private"},
			"date":       time.Now().Unix(),
			"text":       "oi",
		},
	}
	body, err := json.Marshal(update)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/"+ch.WebhookToken, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", ch.WebhookToken)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var convID string
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT id FROM conversations WHERE channel_id = $1`, ch.ID).Scan(&convID))
	_, err = testPool.Exec(context.Background(),
		`UPDATE conversations
		    SET is_bot_paused = TRUE, status = $2, updated_at = NOW() - INTERVAL '2 hours'
		  WHERE id = $1`, convID, string(domain.ConversationPendingHuman))
	require.NoError(t, err)
	return convID
}
