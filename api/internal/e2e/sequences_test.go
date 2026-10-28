package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Cadences against a real database. The rules that matter here are the ones
// that protect somebody outside the system: the customer's WhatsApp number,
// and the lead who already answered.

func emailStep(body string) map[string]any {
	return map[string]any{"kind": "email", "body": body}
}

func createSequence(t *testing.T, token string, steps []map[string]any, active bool) map[string]any {
	t.Helper()
	w := doRequest("PUT", "/v1/sequences/new", map[string]any{
		"name": "Follow-up padrão", "is_active": active, "steps": steps,
	}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return parseJSON(t, w)
}

func TestSequences_CreateListDelete(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	created := createSequence(t, token, []map[string]any{
		emailStep("Bom dia!"),
		{"kind": "wait", "delay_hours": 72},
		emailStep("Voltando ao assunto"),
	}, false)
	id := created["id"].(string)
	// Not configurable, and returned so the editor can state the rule at the
	// top of the screen.
	assert.Equal(t, true, created["stop_on_reply"])

	assert.Len(t, parseJSONArray(t, doRequest("GET", "/v1/sequences", nil, token)), 1)

	require.Equal(t, http.StatusNoContent,
		doRequest("DELETE", "/v1/sequences/"+id, nil, token).Code)
	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/sequences", nil, token)))
}

// A WhatsApp step this early is by definition cold. Spending a template send
// — and a slice of the number's standing with Meta — on somebody who has not
// reacted to two emails is paying the most for the least likely.
func TestSequences_RefuseToOpenWithWhatsApp(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	w := doRequest("PUT", "/v1/sequences/new", map[string]any{
		"name":  "Agressiva",
		"steps": []map[string]any{{"kind": "whatsapp", "template_id": "qualquer"}},
	}, token)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "OPENS_WITH_WHATSAPP")
}

// The account has no approved templates, so every WhatsApp step fails —
// which is the safe direction: an unapproved template is undeliverable and
// sending against one is what gets a number banned.
func TestSequences_WhatsAppNeedsAnApprovedTemplate(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	w := doRequest("PUT", "/v1/sequences/new", map[string]any{
		"name": "Com WhatsApp",
		"steps": []map[string]any{
			emailStep("um"), emailStep("dois"),
			{"kind": "whatsapp", "template_id": "nao_aprovado"},
		},
	}, token)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "WHATSAPP_COLD_NEEDS_TEMPLATE")
}

// Every violation at once: an editor that reports one problem per save turns
// a five-step cadence into five round trips.
func TestSequences_ReportEveryViolationTogether(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	w := doRequest("PUT", "/v1/sequences/new", map[string]any{
		"name": "Quebrada",
		"steps": []map[string]any{
			{"kind": "email"},
			{"kind": "human_task"},
			{"kind": "wait", "delay_hours": 24},
		},
	}, token)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "STEP_HAS_NO_CONTENT")
	assert.Contains(t, body, "TASK_NEEDS_TITLE")
	assert.Contains(t, body, "ENDS_WITH_WAIT")
}

// ── Enrollment ────────────────────────────────────────────────────────────

func seedLeadForSequence(t *testing.T, accountID string) string {
	t.Helper()
	ctx := context.Background()

	funnelID := ulid.Make().String()
	_, err := testPool.Exec(ctx,
		`INSERT INTO sales_funnels (id, account_id, name, created_at, updated_at)
		 VALUES ($1, $2, 'Vendas', NOW(), NOW())`, funnelID, accountID)
	require.NoError(t, err)

	// leads.contact_id is an FK, so the contact has to exist first.
	contactID := ulid.Make().String()
	_, err = testPool.Exec(ctx,
		`INSERT INTO contacts (id, account_id, external_id, name, created_at, updated_at)
		 VALUES ($1, $2, '+5511999990000', 'Clínica Bem Viver', NOW(), NOW())`,
		contactID, accountID)
	require.NoError(t, err)

	leadID := ulid.Make().String()
	_, err = testPool.Exec(ctx,
		`INSERT INTO leads (id, account_id, funnel_id, contact_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'prospected', NOW(), NOW())`,
		leadID, accountID, funnelID, contactID)
	require.NoError(t, err)
	return leadID
}

func TestSequences_EnrollALead(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	seq := createSequence(t, token, []map[string]any{emailStep("Bom dia!")}, true)
	leadID := seedLeadForSequence(t, accountID)

	w := doRequest("POST", "/v1/sequences/"+seq["id"].(string)+"/enroll",
		map[string]any{"lead_id": leadID}, token)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "active", parseJSON(t, w)["status"])
}

// A lead in two cadences gets two follow-ups a week from the same company,
// which reads as disorganised at best.
func TestSequences_ALeadIsInAtMostOneCadence(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	first := createSequence(t, token, []map[string]any{emailStep("um")}, true)
	leadID := seedLeadForSequence(t, accountID)

	require.Equal(t, http.StatusOK,
		doRequest("POST", "/v1/sequences/"+first["id"].(string)+"/enroll",
			map[string]any{"lead_id": leadID}, token).Code)

	w := doRequest("POST", "/v1/sequences/"+first["id"].(string)+"/enroll",
		map[string]any{"lead_id": leadID}, token)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "LEAD_ALREADY_ENROLLED")
}

// Half a cadence applied to a real prospect is worse than none.
func TestSequences_CannotEnrollOnAnInactiveCadence(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	seq := createSequence(t, token, []map[string]any{emailStep("um")}, false)
	leadID := seedLeadForSequence(t, accountID)

	w := doRequest("POST", "/v1/sequences/"+seq["id"].(string)+"/enroll",
		map[string]any{"lead_id": leadID}, token)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "SEQUENCE_INACTIVE")
}

func TestSequences_PauseStopsTheEnrollment(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	seq := createSequence(t, token, []map[string]any{emailStep("um")}, true)
	leadID := seedLeadForSequence(t, accountID)
	require.Equal(t, http.StatusOK,
		doRequest("POST", "/v1/sequences/"+seq["id"].(string)+"/enroll",
			map[string]any{"lead_id": leadID}, token).Code)

	require.Equal(t, http.StatusNoContent,
		doRequest("POST", "/v1/leads/"+leadID+"/sequence/pause", nil, token).Code)

	var status string
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT status FROM sequence_enrollments WHERE lead_id = $1`, leadID).
		Scan(&status))
	assert.Equal(t, "paused", status)
	// Paused means the worker never looks at it again.
	var nextRun *string
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT next_run_at::text FROM sequence_enrollments WHERE lead_id = $1`, leadID).
		Scan(&nextRun))
	assert.Nil(t, nextRun)
}

// One account's cadences are not another's.
func TestSequences_AreScopedToTheAccount(t *testing.T) {
	cleanDB(t)
	owner := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")
	createSequence(t, owner, []map[string]any{emailStep("um")}, false)
	other := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/sequences", nil, other)))
}

// THE rule. A cadence that keeps firing after a human answered is not
// persistence — it is a machine talking over somebody, and it is the fastest
// way to lose a deal that was working.
func TestSequences_AReplyStopsTheCadence(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	token := tokenForSeededAccount(t, ch.AccountID, "admin")
	seq := createSequence(t, token, []map[string]any{emailStep("Bom dia!")}, true)

	// A funnel bound to THIS channel, so the inbound pipeline can resolve
	// the lead from the message's sender.
	funnelID := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO sales_funnels (id, account_id, name, channel_id, created_at, updated_at)
		 VALUES ($1, $2, 'Vendas', $3, NOW(), NOW())`,
		funnelID, ch.AccountID, ch.ID)
	require.NoError(t, err)

	// contacts.id is TEXT and, for a contact reached by email, IS the
	// address — the legacy identity shape the approach path relies on
	// (lead.ContactID == external_id). The inbound pipeline resolves the
	// lead by that same value, so the seed has to match it or the reply
	// would create a second, inbound-first lead instead of finding this one.
	const contactID = "dono@clinica.com.br"
	_, err = testPool.Exec(context.Background(),
		`INSERT INTO contacts (id, account_id, external_id, name, created_at, updated_at)
		 VALUES ($1, $2, $1, 'Clínica', NOW(), NOW())`,
		contactID, ch.AccountID)
	require.NoError(t, err)

	leadID := ulid.Make().String()
	_, err = testPool.Exec(context.Background(),
		`INSERT INTO leads (id, account_id, funnel_id, contact_id, status, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,'contacted',NOW(),NOW())`,
		leadID, ch.AccountID, funnelID, contactID)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK,
		doRequest("POST", "/v1/sequences/"+seq["id"].(string)+"/enroll",
			map[string]any{"lead_id": leadID}, token).Code)

	// The lead answers.
	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "dono@clinica.com.br", "to": "comercial@clinica.com.br",
		"subject": "Re: proposta", "text": "Bom dia! Podemos conversar sim.",
	}).Code)

	var status string
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT status FROM sequence_enrollments WHERE lead_id = $1`, leadID).
		Scan(&status))
	// "replied", not "finished": a cadence that stopped because somebody
	// answered is the outcome it exists for, and counting it as completion
	// would hide the only number that matters.
	assert.Equal(t, "replied", status)
}
