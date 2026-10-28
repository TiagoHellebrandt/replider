package e2e

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-M005 end to end: a provider POSTs an inbound email to the channel's
// webhook route and it must become a conversation with a message, attributed
// to a contact resolved BY EMAIL IDENTITY (US-F007) — the whole reason the
// identity refactor came first.

func seedEmailChannel(t *testing.T) *domain.Channel {
	t.Helper()
	accountID := seedIdentityAccount(t)
	ch := &domain.Channel{
		ID:           ulid.Make().String(),
		AccountID:    accountID,
		Name:         "Comercial",
		Type:         "email",
		WebhookToken: ulid.Make().String(),
		IsActive:     true,
	}
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO channels (id, account_id, name, provider, webhook_token, is_active, credentials, created_at, updated_at)
		 VALUES ($1, $2, $3, 'email', $4, true,
		         '{"from_address":"comercial@clinica.com.br","smtp_host":"smtp.x.com","smtp_username":"u","smtp_password":"p"}',
		         NOW(), NOW())`,
		ch.ID, ch.AccountID, ch.Name, ch.WebhookToken)
	require.NoError(t, err)
	return ch
}

// e2eEmailWebhookSecret is what the suite's router is wired with. The inbound
// mail endpoint verifies a Standard Webhooks signature and fails CLOSED, so
// every email test signs — the same scheme the provider uses, not a stub.
const e2eEmailWebhookSecret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"

func postInboundEmail(t *testing.T, token string, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/"+token, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	signInboundEmail(t, req, body)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	return w
}

// signInboundEmail stamps what the provider would send.
func signInboundEmail(t *testing.T, req *http.Request, body []byte) {
	t.Helper()
	key, err := base64.StdEncoding.DecodeString(
		strings.TrimPrefix(e2eEmailWebhookSecret, "whsec_"))
	require.NoError(t, err)
	id, ts := "msg_e2e", strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + ts + "."))
	mac.Write(body)
	req.Header.Set("svix-id", id)
	req.Header.Set("svix-timestamp", ts)
	req.Header.Set("svix-signature", "v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}

func TestEmailInbound_CreatesConversationAndResolvesContactByEmail(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	ch := seedEmailChannel(t)

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":        "Setor de Compras <Compras@Hospital.com.BR>",
		"to":          "comercial@clinica.com.br",
		"subject":     "Re: Orçamento de equipamentos",
		"text":        "Bom dia! Pode enviar a proposta?\n\nEm 12 de julho, Comercial escreveu:\n> segue material",
		"message_id":  "<abc@hospital.com.br>",
		"in_reply_to": "<xyz@clinica.com.br>",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// A conversation exists for this sender on this channel.
	var convID, contactID string
	err := testPool.QueryRow(ctx,
		`SELECT id, contact_id FROM conversations WHERE channel_id = $1`, ch.ID).Scan(&convID, &contactID)
	require.NoError(t, err)
	assert.Equal(t, "compras@hospital.com.br", contactID,
		"the sender identity on an email channel is the address, lowercased")

	// The message landed with the quoted history stripped — otherwise every
	// reply re-feeds the whole thread to the AI.
	var content string
	var metadata []byte
	err = testPool.QueryRow(ctx,
		`SELECT content, metadata FROM messages WHERE conversation_id = $1 AND direction = 'inbound'`,
		convID).Scan(&content, &metadata)
	require.NoError(t, err)
	assert.Equal(t, "Bom dia! Pode enviar a proposta?", content)
	assert.NotContains(t, content, "segue material")

	// Threading headers survive onto the row, so the reply can thread.
	var meta map[string]any
	require.NoError(t, json.Unmarshal(metadata, &meta))
	assert.Equal(t, "<abc@hospital.com.br>", meta["message_id"])
	assert.Equal(t, "Re: Orçamento de equipamentos", meta["subject"])

	// ...and the identity index knows who this is (US-F007).
	identities := postgres.NewContactIdentityRepository(testPool)
	resolved, err := identities.ContactIDByValue(ctx, ch.AccountID, domain.IdentityKindEmail, "compras@hospital.com.br")
	require.NoError(t, err)
	require.NotEmpty(t, resolved)

	contacts := postgres.NewContactRepository(testPool)
	contact, err := contacts.GetByID(ctx, resolved, ch.AccountID)
	require.NoError(t, err)
	require.NotNil(t, contact)
	require.NotNil(t, contact.Email)
	assert.Equal(t, "compras@hospital.com.br", *contact.Email)
	require.NotNil(t, contact.Name)
	assert.Equal(t, "Setor de Compras", *contact.Name)
}

// A provider retry of the same Message-ID must not create a second message —
// mail providers retry aggressively on any non-2xx.
func TestEmailInbound_DedupesProviderRetries(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	ch := seedEmailChannel(t)

	payload := map[string]any{
		"from":       "compras@hospital.com.br",
		"subject":    "Orçamento",
		"text":       "primeira mensagem",
		"message_id": "<dup@hospital.com.br>",
	}
	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, payload).Code)
	require.Equal(t, http.StatusOK, postInboundEmail(t, ch.WebhookToken, payload).Code)

	var count int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM messages m JOIN conversations c ON c.id = m.conversation_id
		 WHERE c.channel_id = $1 AND m.direction = 'inbound'`, ch.ID).Scan(&count))
	assert.Equal(t, 1, count)
}

// An unattributable payload is ACKED, not 500-ed: a provider that gets a 5xx
// retries forever, and there is nothing to retry into.
func TestEmailInbound_AcksUnusablePayload(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{"subject": "sem remetente", "text": "oi"})
	assert.Equal(t, http.StatusOK, w.Code)

	var count int
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM conversations WHERE channel_id = $1`, ch.ID).Scan(&count))
	assert.Zero(t, count)
}

// V4: an HTML-only reply is stored as text, with script content gone — the
// dashboard renders this string.
func TestEmailInbound_StoresHTMLBodyAsSanitizedText(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":       "compras@hospital.com.br",
		"subject":    "Proposta",
		"html":       "<div><p>Segue o aceite.</p><script>alert('xss')</script></div>",
		"message_id": "<html@hospital.com.br>",
	})
	require.Equal(t, http.StatusOK, w.Code)

	var content string
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT m.content FROM messages m JOIN conversations c ON c.id = m.conversation_id
		 WHERE c.channel_id = $1`, ch.ID).Scan(&content))

	assert.Contains(t, content, "Segue o aceite.")
	assert.NotContains(t, content, "<")
	assert.NotContains(t, content, "alert")
}

// The door that had no lock, end to end against a real database.
//
// Until 15/09/2026 this endpoint verified nothing, so whoever held the channel
// URL could post mail from any sender — and because the webhook path fills the
// auth verdict from the POST BODY (the IMAP path verifies DKIM against the raw
// message; this one cannot), the forgery could assert its own `spf=pass` and be
// graded clean by the risk layer built to catch supplier impersonation.
//
// Asserted against the DB, not just the status code: a 403 that still wrote a
// conversation would be theatre.
func TestEmailInbound_UnsignedDeliveryIsRefusedAndStoresNothing(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	payload := map[string]any{
		"from":    "compras@fornecedor.com.br",
		"to":      "vendas@cliente.com.br",
		"subject": "Atualizacao de dados bancarios",
		"text":    "Segue a nova conta para o proximo pagamento.",
		// The forgery asserting its own clean bill of health.
		"spf": "pass", "dkim": "pass", "dmarc": "pass",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/"+ch.WebhookToken, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No signature at all — the shape the endpoint used to accept.
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	var conversations, messages int
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM conversations WHERE channel_id = $1`, ch.ID).Scan(&conversations))
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM messages`).Scan(&messages))
	assert.Zero(t, conversations, "a forged delivery created a conversation")
	assert.Zero(t, messages, "a forged delivery stored a message")
}

// And a signed one still arrives — the gate must not be a wall.
func TestEmailInbound_SignedDeliveryStillArrives(t *testing.T) {
	cleanDB(t)
	ch := seedEmailChannel(t)

	w := postInboundEmail(t, ch.WebhookToken, map[string]any{
		"from":    "compras@hospital.com.br",
		"to":      "vendas@cliente.com.br",
		"subject": "Proposta",
		"text":    "Podem enviar a proposta?",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var conversations int
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM conversations WHERE channel_id = $1`, ch.ID).Scan(&conversations))
	assert.Equal(t, 1, conversations)
}
