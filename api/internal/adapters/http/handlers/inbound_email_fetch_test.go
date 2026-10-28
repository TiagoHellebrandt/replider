package handlers

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

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mail nobody wrote must not enter the pipeline, on EITHER arrival route.
//
// IMAP is the route production mail actually takes, and it used to skip the
// delivery-report check the webhook ran: every MAILER-DAEMON and every
// "estou de férias" became a contact, a lead in "responded", and the end of
// a cadence — while the dead address was never suppressed.

type fakeSuppressionStore struct {
	suppressed []*domain.EmailSuppression
}

func (f *fakeSuppressionStore) Suppress(_ context.Context, s *domain.EmailSuppression) error {
	f.suppressed = append(f.suppressed, s)
	return nil
}

func (f *fakeSuppressionStore) IsSuppressed(context.Context, string, string) (bool, error) {
	return false, nil
}

func (f *fakeSuppressionStore) List(context.Context, string, int, int) ([]*domain.EmailSuppression, error) {
	return f.suppressed, nil
}

func (f *fakeSuppressionStore) Release(context.Context, string, string) error { return nil }

// emailIngestFixture wires a handler whose persistence mocks carry NO
// expectations: any write reaching them fails the test, which is exactly the
// assertion — a report or an auto-reply stores nothing.
func emailIngestFixture(t *testing.T) (*InboundWebhookHandler, *MockChannelRepo, *MockConversationRepo, *MockMessageRepo, *fakeSuppressionStore) {
	t.Helper()
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)
	handler, _ := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	sup := &fakeSuppressionStore{}
	handler.SetBounceUseCase(usecases.NewEmailBounceUseCase(sup, msgRepo, nil))
	return handler, channelRepo, convRepo, msgRepo, sup
}

func emailChannelFixture() *domain.Channel {
	return &domain.Channel{
		ID: "ch-mail-1", AccountID: testAccountID, Type: "email",
		WebhookToken: "mail-token-1", IsActive: true,
		Config: map[string]any{"from_address": "comercial@fornecedor.com.br"},
	}
}

// A hard bounce read off the mailbox: recorded (the address is suppressed),
// and nothing about it is stored as a message.
func TestIngestFetchedEmail_ADeliveryReportIsRecordedNotStored(t *testing.T) {
	handler, _, convRepo, msgRepo, sup := emailIngestFixture(t)

	raw := "From: Mail Delivery Subsystem <mailer-daemon@googlemail.com>\r\n" +
		"To: comercial@fornecedor.com.br\r\n" +
		"Subject: Delivery Status Notification (Failure)\r\n" +
		"Message-Id: <dsn-1@googlemail.com>\r\n" +
		"Auto-Submitted: auto-replied\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Final-Recipient: rfc822; dono@clinicafechada.com.br\r\n" +
		"Action: failed\r\n" +
		"Status: 5.1.1\r\n" +
		"Diagnostic-Code: smtp; 550 5.1.1 The email account that you tried to reach does not exist\r\n"

	stored, err := handler.IngestFetchedEmail(context.Background(), emailChannelFixture(), []byte(raw))

	require.NoError(t, err)
	assert.False(t, stored)
	require.Len(t, sup.suppressed, 1, "the dead address was not suppressed")
	assert.Equal(t, "dono@clinicafechada.com.br", sup.suppressed[0].Address)
	assert.Equal(t, domain.SuppressionHardBounce, sup.suppressed[0].Reason)
	convRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// An out-of-office is dropped: no conversation, no contact, no lead — and no
// suppression either, because the address works.
func TestIngestFetchedEmail_AnAutoReplyIsDropped(t *testing.T) {
	handler, _, convRepo, msgRepo, sup := emailIngestFixture(t)

	raw := "From: Ana Souza <ana@clinicaaurora.com.br>\r\n" +
		"To: comercial@fornecedor.com.br\r\n" +
		"Subject: Resposta automática: proposta de luvas\r\n" +
		"Message-Id: <ooo-1@clinicaaurora.com.br>\r\n" +
		"Auto-Submitted: auto-replied\r\n" +
		"Precedence: bulk\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Estou de férias até 15/09. Retorno assim que possível.\r\n"

	stored, err := handler.IngestFetchedEmail(context.Background(), emailChannelFixture(), []byte(raw))

	require.NoError(t, err)
	assert.False(t, stored)
	assert.Empty(t, sup.suppressed)
	convRepo.AssertNotCalled(t, "FindByContact", mock.Anything, mock.Anything, mock.Anything)
	convRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// The webhook route runs the same gate: a provider that forwards the raw
// headers gets its autoresponders acked as "ignored", never stored.
func TestInboundWebhook_Email_AnAutoReplyIsAckedAndDropped(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)
	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)
	channelRepo.On("GetByWebhookToken", mock.Anything, "mail-token-1").Return(emailChannelFixture(), nil)

	body, _ := json.Marshal(map[string]any{
		"from":    "Ana Souza <ana@clinicaaurora.com.br>",
		"subject": "Resposta automática: proposta",
		"text":    "Estou de férias até 15/09.",
		"headers": map[string]string{"Auto-Submitted": "auto-replied"},
	})
	req := httptest.NewRequest(http.MethodPost, "/webhooks/mail-token-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	signEmailWebhook(t, req, body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "ignored", resp["status"])
	convRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// And a delivery report over the webhook keeps its ack shape: the provider
// sees "bounce_recorded" with the kind, exactly as before the gate was shared.
func TestInboundWebhook_Email_ADeliveryReportKeepsItsAck(t *testing.T) {
	handler, channelRepo, convRepo, msgRepo, sup := emailIngestFixture(t)
	router := setupTestRouter()
	router.POST("/webhooks/:channel_token", handler.HandleInboundWebhook)
	channelRepo.On("GetByWebhookToken", mock.Anything, "mail-token-1").Return(emailChannelFixture(), nil)

	body, _ := json.Marshal(map[string]any{
		"from":    "Mail Delivery Subsystem <mailer-daemon@googlemail.com>",
		"subject": "Delivery Status Notification (Failure)",
		"text": "Final-Recipient: rfc822; dono@clinicafechada.com.br\n" +
			"Action: failed\nStatus: 5.1.1\n" +
			"Diagnostic-Code: smtp; 550 5.1.1 User unknown\n",
	})
	req := httptest.NewRequest(http.MethodPost, "/webhooks/mail-token-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	signEmailWebhook(t, req, body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "bounce_recorded", resp["status"])
	assert.Equal(t, string(domain.BounceHard), resp["kind"])
	require.Len(t, sup.suppressed, 1)
	convRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// testEmailWebhookSecret is the platform-wide inbound-mail signing secret the
// handler tests run with.
const testEmailWebhookSecret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"

// signEmailWebhook stamps the Standard Webhooks headers a provider would send.
//
// It signs with the same scheme the verifier checks rather than stubbing the
// check out: the point of these tests is that a real delivery still arrives
// after the endpoint stopped accepting unsigned ones.
func signEmailWebhook(t *testing.T, req *http.Request, body []byte) {
	t.Helper()
	key, err := base64.StdEncoding.DecodeString(
		strings.TrimPrefix(testEmailWebhookSecret, "whsec_"))
	if err != nil {
		t.Fatalf("test secret: %v", err)
	}
	id, ts := "msg_test", strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + ts + "."))
	mac.Write(body)
	req.Header.Set("svix-id", id)
	req.Header.Set("svix-timestamp", ts)
	req.Header.Set("svix-signature", "v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}

// The door that had no lock. Until 15/09/2026 this endpoint verified nothing,
// so whoever held the channel URL could post an e-mail from any sender — and
// because the webhook path fills InboundEmail.Auth from the POST BODY, the
// forgery could assert its own `spf=pass` and be graded clean by the very risk
// layer built to catch supplier impersonation.
func TestInboundWebhook_Email_RefusesWhatNobodySigned(t *testing.T) {
	body := []byte(`{"from":"compras@fornecedor.com.br","subject":"Dados bancarios novos",` +
		`"text":"Segue a conta atualizada","spf":"pass","dkim":"pass","dmarc":"pass"}`)

	cases := map[string]func(*http.Request){
		"no signature at all": func(*http.Request) {},
		"signature from another secret": func(req *http.Request) {
			key, _ := base64.StdEncoding.DecodeString("bm90IHRoZSByaWdodCBrZXkgYXQgYWxs")
			ts := strconv.FormatInt(time.Now().Unix(), 10)
			mac := hmac.New(sha256.New, key)
			mac.Write([]byte("msg_x." + ts + "."))
			mac.Write(body)
			req.Header.Set("svix-id", "msg_x")
			req.Header.Set("svix-timestamp", ts)
			req.Header.Set("svix-signature", "v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		},
		"body swapped after signing": func(req *http.Request) {
			signEmailWebhook(t, req, []byte(`{"from":"compras@fornecedor.com.br","text":"conta antiga"}`))
		},
	}

	for name, stamp := range cases {
		t.Run(name, func(t *testing.T) {
			channelRepo := new(MockChannelRepo)
			convRepo := new(MockConversationRepo)
			msgRepo := new(MockMessageRepo)
			agentRepo := new(MockAgentRepo)
			_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)
			channelRepo.On("GetByWebhookToken", mock.Anything, "mail-token-1").
				Return(emailChannelFixture(), nil)

			req := httptest.NewRequest(http.MethodPost, "/webhooks/mail-token-1", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			stamp(req)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code, "forged mail was accepted")
			// And it must not have reached the pipeline at all — a 403 that
			// still stored the message would be theatre.
			convRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
			msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		})
	}
}

// Fail CLOSED: with no secret configured anywhere, a delivery is refused
// rather than waved through. Same policy the antivirus gate already uses.
func TestInboundWebhook_Email_NoSecretConfiguredRefusesEverything(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)
	handler, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)
	handler.SetEmailWebhookSecret("") // as if nobody had set EMAIL_WEBHOOK_SECRET
	channelRepo.On("GetByWebhookToken", mock.Anything, "mail-token-1").
		Return(emailChannelFixture(), nil)

	body := []byte(`{"from":"a@b.com","text":"oi"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/mail-token-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	signEmailWebhook(t, req, body) // even a valid signature cannot help
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
