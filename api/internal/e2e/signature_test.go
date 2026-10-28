package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// Electronic signature against a real database (§6.11, G7).
//
// The provider signs nothing, so the callback route is public with only an
// unguessable token. Everything below is about that being safe: a forged
// `doc_signed` causes an authenticated READ, and a real one produces exactly one
// Sale — because a duplicate sale is a duplicate commission.

func configureSignature(t *testing.T, token string) string {
	t.Helper()
	w := doRequest("PUT", "/v1/signature/settings",
		map[string]any{"api_token": "provider-tok-123"}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := parseJSON(t, w)

	// The token never comes back — it is the customer's credential at their
	// provider — but the screen learns the integration is live.
	assert.Nil(t, body["api_token"])
	assert.Equal(t, true, body["has_token"])
	// And the URL they paste into the provider's webhook settings.
	callback, _ := body["callback_url"].(string)
	require.Contains(t, callback, "/v1/webhooks/signature/")

	webhookToken, _ := body["webhook_token"].(string)
	require.NotEmpty(t, webhookToken)
	return webhookToken
}

func sendForSignature(t *testing.T, token, contactID string, valueCents int64) map[string]any {
	t.Helper()
	// A stored PDF the fake object store can serve back.
	require.NoError(t, testAttachmentStore.UploadBytes(context.Background(),
		"contracts/e2e/signature.pdf", []byte("%PDF-1.7 contract"), "application/pdf"))

	w := doRequest("POST", "/v1/signature/requests", map[string]any{
		"object_path": "contracts/e2e/signature.pdf",
		"title":       "Contrato de fornecimento",
		"contact_id":  contactID,
		"value_cents": valueCents,
		"signers": []map[string]any{
			{"name": "Dra. Ana", "email": "ana@clinica.com", "phone": "+5565999990000"},
		},
	}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return parseJSON(t, w)
}

func TestSignature_SendTracksTheDocumentAndTheLink(t *testing.T) {
	cleanDB(t)
	e2eSignatureGateway.reset()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	contactID := seedSaleContact(t, accountID)

	// Nothing configured yet: the integration is missing, which is a 503 pointing
	// at Settings rather than a 400 about the request.
	unconfigured := doRequest("POST", "/v1/signature/requests", map[string]any{
		"object_path": "x", "title": "y",
		"signers": []map[string]any{{"name": "A", "email": "a@b.c"}},
	}, token)
	assert.Equal(t, http.StatusServiceUnavailable, unconfigured.Code, unconfigured.Body.String())
	assert.Equal(t, "SIGNATURE_NOT_CONFIGURED", parseJSON(t, unconfigured)["code"])

	configureSignature(t, token)
	created := sendForSignature(t, token, contactID, 10000000)

	assert.Equal(t, "sent", created["status"])
	assert.NotEmpty(t, created["sent_at"])
	signers, _ := created["signers"].([]any)
	require.Len(t, signers, 1)
	first, _ := signers[0].(map[string]any)
	// The signing link is what a PERSON hands over.
	assert.NotEmpty(t, first["sign_url"])
	// The phone we supplied survives; the provider token only it knows comes back.
	assert.Equal(t, "+5565999990000", first["phone"])
	assert.NotEmpty(t, first["provider_token"])

	assert.Len(t, parseJSONArray(t, doRequest("GET", "/v1/signature/requests", nil, token)), 1)
}

func TestSignature_RefusesWhatCannotBeSent(t *testing.T) {
	cleanDB(t)
	e2eSignatureGateway.reset()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	configureSignature(t, token)

	cases := []struct {
		name string
		body map[string]any
		code int
	}{
		{"no signers", map[string]any{
			"object_path": "contracts/e2e/signature.pdf", "title": "x",
		}, http.StatusBadRequest},
		{"signer nobody can notify", map[string]any{
			"object_path": "contracts/e2e/signature.pdf", "title": "x",
			"signers": []map[string]any{{"name": "Sem contato"}},
		}, http.StatusBadRequest},
		{"no document", map[string]any{
			"title":   "x",
			"signers": []map[string]any{{"name": "A", "email": "a@b.c"}},
		}, http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequest("POST", "/v1/signature/requests", tc.body, token)
			assert.Equal(t, tc.code, w.Code, w.Body.String())
		})
	}
}

// The security spine, end to end: the callback is a hint, never a fact.
func TestSignature_ForgedCallbackRegistersNothing(t *testing.T) {
	cleanDB(t)
	e2eSignatureGateway.reset()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	contactID := seedSaleContact(t, accountID)
	webhookToken := configureSignature(t, token)

	created := sendForSignature(t, token, contactID, 10000000)
	id, _ := created["id"].(string)

	// The provider's API still says "out for signature"; the callback claims it was
	// signed. Anybody who learns the URL can send this.
	before := e2eSignatureGateway.calls()
	forged := doRequest("POST", "/v1/webhooks/signature/"+webhookToken, map[string]any{
		"event_type": "doc_signed", "external_id": id,
	}, "")
	// Always 200: a 4xx makes a provider retry or disable the hook, and neither
	// helps when the cause is a document we do not know about.
	require.Equal(t, http.StatusOK, forged.Code, forged.Body.String())

	// It caused an AUTHENTICATED READ...
	assert.Greater(t, e2eSignatureGateway.calls(), before)
	// ...and changed nothing.
	after := parseJSON(t, doRequest("GET", "/v1/signature/requests/"+id, nil, token))
	assert.Equal(t, "sent", after["status"])
	assert.Nil(t, after["sale_id"])
	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/sales", nil, token)))

	// An unknown webhook token is accepted and ignored, saying nothing about which
	// accounts exist.
	unknown := doRequest("POST", "/v1/webhooks/signature/01JUNKJUNKJUNKJUNKJUNKJUNK",
		map[string]any{"event_type": "doc_signed", "external_id": id}, "")
	assert.Equal(t, http.StatusOK, unknown.Code)
	assert.Equal(t, "ignored", parseJSON(t, unknown)["status"])
}

func TestSignature_SignedArchivesAndRegistersExactlyOneSale(t *testing.T) {
	cleanDB(t)
	e2eSignatureGateway.reset()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	contactID := seedSaleContact(t, accountID)
	webhookToken := configureSignature(t, token)

	created := sendForSignature(t, token, contactID, 10000000)
	id, _ := created["id"].(string)

	// NOW the provider's API says it was signed.
	e2eSignatureGateway.setTruth(domain.SignatureSigned, "https://cdn.test/signed.pdf")

	// Three deliveries of the same event: providers retry.
	for i := 0; i < 3; i++ {
		w := doRequest("POST", "/v1/webhooks/signature/"+webhookToken, map[string]any{
			"event_type": "doc_signed", "external_id": id,
		}, "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}

	detail := parseJSON(t, doRequest("GET", "/v1/signature/requests/"+id, nil, token))
	assert.Equal(t, "signed", detail["status"])
	assert.NotEmpty(t, detail["signed_at"])
	// The signed copy lives in OUR bucket: a document that lives only in the
	// provider's account is one the customer loses when they leave that provider.
	archived, _ := detail["signed_object_path"].(string)
	require.NotEmpty(t, archived)
	stored, err := testAttachmentStore.DownloadObject(context.Background(), archived, 1<<20)
	require.NoError(t, err)
	assert.Contains(t, string(stored), "signed copy")

	// Exactly ONE sale, and the request points at it.
	sales := parseJSONArray(t, doRequest("GET", "/v1/sales", nil, token))
	require.Len(t, sales, 1, "a retried webhook must not become a second sale")
	sale, _ := sales[0].(map[string]any)
	assert.EqualValues(t, 10000000, sale["gross_cents"])
	assert.Equal(t, contactID, sale["contact_id"])
	saleID, _ := sale["id"].(string)
	assert.Equal(t, saleID, detail["sale_id"])

	// The portfolio sees it, which is the whole point of registering the sale.
	portfolio := parseJSONArray(t, doRequest("GET", "/v1/portfolio", nil, token))
	assert.NotEmpty(t, portfolio)
}

func TestSignature_SignedWithNoValueIsTrackedButNotSold(t *testing.T) {
	cleanDB(t)
	e2eSignatureGateway.reset()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	contactID := seedSaleContact(t, accountID)
	webhookToken := configureSignature(t, token)

	// An NDA: signed, worth nothing recorded.
	created := sendForSignature(t, token, contactID, 0)
	id, _ := created["id"].(string)
	e2eSignatureGateway.setTruth(domain.SignatureSigned, "https://cdn.test/signed.pdf")

	require.Equal(t, http.StatusOK,
		doRequest("POST", "/v1/webhooks/signature/"+webhookToken, map[string]any{
			"event_type": "doc_signed", "external_id": id,
		}, "").Code)

	detail := parseJSON(t, doRequest("GET", "/v1/signature/requests/"+id, nil, token))
	assert.Equal(t, "signed", detail["status"])
	assert.Nil(t, detail["sale_id"])
	// A signed document with no value is not a sale, and inventing one would put a
	// number in somebody's commission.
	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/sales", nil, token)))
}

func TestSignature_RefreshIsTheManualHalfOfTheSameRead(t *testing.T) {
	cleanDB(t)
	e2eSignatureGateway.reset()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	contactID := seedSaleContact(t, accountID)
	configureSignature(t, token)

	created := sendForSignature(t, token, contactID, 5000000)
	id, _ := created["id"].(string)

	// The customer opened it and did not sign — the state that changes a follow-up
	// call.
	e2eSignatureGateway.setTruth(domain.SignatureViewed, "")
	viewed := parseJSON(t, doRequest("POST", "/v1/signature/requests/"+id+"/refresh", nil, token))
	assert.Equal(t, "viewed", viewed["status"])
	assert.NotEmpty(t, viewed["viewed_at"])

	// Then they sign, and the button gets there before any webhook does.
	e2eSignatureGateway.setTruth(domain.SignatureSigned, "https://cdn.test/signed.pdf")
	signed := parseJSON(t, doRequest("POST", "/v1/signature/requests/"+id+"/refresh", nil, token))
	assert.Equal(t, "signed", signed["status"])
	assert.NotEmpty(t, signed["sale_id"])

	// A late "viewed" must not walk it backwards: terminal is terminal.
	e2eSignatureGateway.setTruth(domain.SignatureViewed, "")
	late := parseJSON(t, doRequest("POST", "/v1/signature/requests/"+id+"/refresh", nil, token))
	assert.Equal(t, "signed", late["status"])
	assert.Len(t, parseJSONArray(t, doRequest("GET", "/v1/sales", nil, token)), 1)
}

func TestSignature_TenancyAndRole(t *testing.T) {
	cleanDB(t)
	e2eSignatureGateway.reset()
	accountA := seedIdentityAccount(t)
	accountB := seedIdentityAccount(t)
	tokenA := tokenForSeededAccount(t, accountA, "admin")
	tokenB := tokenForSeededAccount(t, accountB, "admin")
	attendant := tokenForSeededAccount(t, accountA, "attendant")
	contactID := seedSaleContact(t, accountA)

	webhookToken := configureSignature(t, tokenA)
	created := sendForSignature(t, tokenA, contactID, 10000000)
	id, _ := created["id"].(string)

	// Another company's signature is not readable or refreshable.
	assert.Equal(t, http.StatusNotFound,
		doRequest("GET", "/v1/signature/requests/"+id, nil, tokenB).Code)
	assert.Equal(t, http.StatusNotFound,
		doRequest("POST", "/v1/signature/requests/"+id+"/refresh", nil, tokenB).Code)
	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/signature/requests", nil, tokenB)))
	// And account B's settings are its own: a fresh webhook token, not A's.
	otherToken := configureSignature(t, tokenB)
	assert.NotEqual(t, webhookToken, otherToken)

	// The module is admin-only: sending spends the customer's own quota and can
	// register a Sale that lands in somebody's commission.
	assert.Equal(t, http.StatusForbidden,
		doRequest("GET", "/v1/signature/settings", nil, attendant).Code)
	assert.Equal(t, http.StatusForbidden,
		doRequest("PUT", "/v1/signature/settings", map[string]any{"api_token": "x"}, attendant).Code)
	assert.Equal(t, http.StatusForbidden,
		doRequest("POST", "/v1/signature/requests", map[string]any{
			"object_path": "p", "title": "t",
			"signers": []map[string]any{{"name": "A", "email": "a@b.c"}},
		}, attendant).Code)
	assert.Equal(t, http.StatusForbidden,
		doRequest("GET", "/v1/signature/requests", nil, attendant).Code)
}
