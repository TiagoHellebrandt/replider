package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Triage against a real database, because the two things that matter are
// stored state (does the category survive?) and SQL (does the filter run
// before LIMIT/OFFSET?).

func categoryOf(t *testing.T, channelID, contactID string) string {
	t.Helper()
	var cat *string
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT inbox_category FROM conversations
		  WHERE channel_id = $1 AND contact_id = $2`, channelID, contactID).Scan(&cat))
	if cat == nil {
		return ""
	}
	return *cat
}

func TestInboxTriage_FilesAnOrdinaryMessage(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":    "dono@clinica.com.br",
		"to":      "comercial@clinica.com.br",
		"subject": "Bom dia",
		"text":    "Tudo bem? Queria conversar.",
	}).Code)

	// Nothing decisive, and "other" is written rather than left null — a
	// conversation with no category sorts at zero and vanishes under
	// everything.
	assert.Equal(t, "other", categoryOf(t, ch.ID, "dono@clinica.com.br"))
}

// The honest bulk-mail signal, and the reason a newsletter does not sit at
// the top of the inbox all morning.
func TestInboxTriage_ListUnsubscribeMakesItANewsletter(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":    "news@fornecedor.com.br",
		"to":      "comercial@clinica.com.br",
		"subject": "Novidades de janeiro",
		"text":    "Confira nossos lançamentos.",
		"headers": map[string]string{
			"List-Unsubscribe": "<https://fornecedor.com.br/sair>",
		},
	}).Code)

	assert.Equal(t, "newsletter", categoryOf(t, ch.ID, "news@fornecedor.com.br"))
}

// Money moving is its own bucket because it is what a fraudster imitates.
func TestInboxTriage_APaymentInstructionIsFinancial(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":    "financeiro@fornecedor.com.br",
		"to":      "comercial@clinica.com.br",
		"subject": "Cobrança de janeiro",
		"text":    "Segue nossa chave PIX: 12.345.678/0001-95",
	}).Code)

	assert.Equal(t, "financial", categoryOf(t, ch.ID, "financeiro@fornecedor.com.br"))
}

// The filter must run in SQL. Applied in Go over an already-paginated page it
// would answer "nenhum e-mail financeiro" for an inbox that has several,
// just further down.
func TestInboxTriage_CategoryFilterRunsInSQL(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	token := tokenForSeededAccount(t, ch.AccountID, "admin")

	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "a@x.com", "to": "comercial@clinica.com.br",
		"subject": "Oi", "text": "Tudo bem?",
	}).Code)
	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "financeiro@y.com", "to": "comercial@clinica.com.br",
		"subject": "Boleto", "text": "PIX: 12.345.678/0001-95",
	}).Code)

	all := parseJSONArray(t, doRequest("GET", "/v1/conversations?status=open", nil, token))
	require.Len(t, all, 2)

	only := parseJSONArray(t,
		doRequest("GET", "/v1/conversations?status=open&category=financial", nil, token))
	require.Len(t, only, 1)
	assert.Equal(t, "financeiro@y.com", only[0].(map[string]any)["contact_id"])
}

// The value reaches SQL, so an unknown one is a 400 rather than a silently
// empty inbox.
func TestInboxTriage_AnInventedCategoryIsRefused(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	token := tokenForSeededAccount(t, ch.AccountID, "admin")

	w := doRequest("GET", "/v1/conversations?status=open&category=inventada", nil, token)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Chronological order is right for a chat and wrong for a commercial
// mailbox, where the newest thing is usually the least important.
func TestInboxTriage_ImportanceSortPutsTheNewsletterLast(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)
	token := tokenForSeededAccount(t, ch.AccountID, "admin")

	// The financial one first, the newsletter last — so chronological order
	// would put the newsletter on top.
	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "financeiro@y.com", "to": "comercial@clinica.com.br",
		"subject": "Boleto", "text": "PIX: 12.345.678/0001-95",
	}).Code)
	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from": "news@z.com", "to": "comercial@clinica.com.br",
		"subject": "Novidades", "text": "Confira.",
		"headers": map[string]string{"List-Unsubscribe": "<https://z.com/sair>"},
	}).Code)

	byImportance := parseJSONArray(t,
		doRequest("GET", "/v1/conversations?status=open&sort=importance", nil, token))

	require.Len(t, byImportance, 2)
	assert.Equal(t, "financeiro@y.com", byImportance[0].(map[string]any)["contact_id"])
	assert.Equal(t, "news@z.com", byImportance[1].(map[string]any)["contact_id"])
}
