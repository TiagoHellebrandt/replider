package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	igadapter "github.com/replider/api/internal/adapters/channels/instagram"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// US-007: Instagram inbound webhook end-to-end at the handler level —
// realistic Meta payloads through /webhooks/:channel_token, plus the
// hub.challenge GET verification and X-Hub-Signature-256 validation.

const igSenderID = "1254459081234567"
const igMID = "aWdfZAG1faXRlbTox"

// igWebhookBody is a realistic Instagram messaging webhook delivery
// (Instagram object, `messages` field subscription).
func igWebhookBody() []byte {
	return []byte(`{
		"object": "instagram",
		"entry": [{
			"id": "17841400000000001",
			"time": 1720000000,
			"messaging": [{
				"sender": {"id": "` + igSenderID + `"},
				"recipient": {"id": "17841400000000001"},
				"timestamp": 1720000000123,
				"message": {"mid": "` + igMID + `", "text": "Oi! Vocês têm horário amanhã?"}
			}]
		}]
	}`)
}

func igChannel(agentID *string, config any) *domain.Channel {
	return &domain.Channel{
		AccountID:    testAccountID,
		ID:           "ch-ig-1",
		AgentID:      agentID,
		Type:         "instagram",
		WebhookToken: "ig-token-1",
		IsActive:     true,
		Config:       config,
	}
}

func hmacHeader(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// igSignedConfig is the channel config for tests exercising the pipeline
// past the (fail-closed) signature gate via a per-channel app_secret.
const igTestSecret = "tenant-app-secret"

func igSignedConfig() json.RawMessage {
	return json.RawMessage(`{"page_id":"17841400000000001","access_token":"IGQVJ...","app_secret":"` + igTestSecret + `"}`)
}

func postIG(router http.Handler, body []byte, secret string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/webhooks/ig-token-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-Hub-Signature-256", hmacHeader(body, secret))
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// ────────────── IG Inbound — new conversation, no agent (AI skipped) ──────────────

func TestInboundWebhook_Instagram_NewConversation_NoAgent(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	// No agent assigned → human-only channel, ProcessAI must be a no-op.
	channelRepo.On("GetByWebhookToken", mock.Anything, "ig-token-1").
		Return(igChannel(nil, igSignedConfig()), nil)

	// Webhook-retry dedupe lookup on the mid — first delivery → no hit.
	msgRepo.On("GetByExternalID", mock.Anything, "ch-ig-1", igMID).
		Return((*domain.Message)(nil), nil)

	// First DM from this IGSID → conversation is created.
	convRepo.On("FindByContact", mock.Anything, "ch-ig-1", igSenderID).Return(nil, nil)
	convRepo.On("Create", mock.Anything, mock.MatchedBy(func(cv *domain.Conversation) bool {
		return cv.AccountID == testAccountID && cv.ChannelID == "ch-ig-1" &&
			cv.ContactID == igSenderID && cv.AgentID == nil
	})).Return(nil)
	convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
		return m.Direction == "inbound" && m.From == igSenderID &&
			*m.Content == "Oi! Vocês têm horário amanhã?" &&
			m.ExternalMessageID != nil && *m.ExternalMessageID == igMID
	})).Return(true, nil)

	w := postIG(router, igWebhookBody(), igTestSecret)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "processed", resp["status"])
	assert.NotEmpty(t, resp["conversation_id"])

	// AI pipeline skipped: the async goroutine bails on the nil agent
	// before touching the agent repo. Small grace period so a buggy
	// implementation that DOES call it has time to do so.
	time.Sleep(150 * time.Millisecond)
	agentRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)

	channelRepo.AssertExpectations(t)
	convRepo.AssertExpectations(t)
	msgRepo.AssertExpectations(t)
}

// ────────────── IG Inbound — agent assigned → AI pipeline invoked ──────────────

func TestInboundWebhook_Instagram_AIPipelineInvoked(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channelRepo.On("GetByWebhookToken", mock.Anything, "ig-token-1").
		Return(igChannel(strp(testAgentID), igSignedConfig()), nil)

	msgRepo.On("GetByExternalID", mock.Anything, "ch-ig-1", igMID).
		Return((*domain.Message)(nil), nil)

	// Existing conversation, bot ACTIVE → ProcessAI runs on the detached
	// goroutine and its first repo touch is the agent lookup. Returning
	// (nil, nil) makes ProcessAI log "agent not found" and stop — proving
	// the pipeline was entered without wiring a full ProcessMessageUseCase.
	convRepo.On("FindByContact", mock.Anything, "ch-ig-1", igSenderID).Return(&domain.Conversation{
		ID:          "conv-ig-1",
		AccountID:   testAccountID,
		ChannelID:   "ch-ig-1",
		ContactID:   igSenderID,
		Status:      domain.ConversationOpen,
		IsBotPaused: false,
	}, nil)
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-ig-1", testAccountID).Return(nil)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Message")).Return(true, nil)

	aiInvoked := make(chan struct{})
	agentRepo.On("GetByID", mock.Anything, testAgentID).
		Run(func(args mock.Arguments) { close(aiInvoked) }).
		Return(nil, nil)

	w := postIG(router, igWebhookBody(), igTestSecret)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "processed", resp["status"])

	select {
	case <-aiInvoked:
		// AI pipeline reached the agent lookup — invoked as required.
	case <-time.After(2 * time.Second):
		t.Fatal("AI pipeline was not invoked for an agent-assigned Instagram channel")
	}
}

// ────────────── IG Inbound — X-Hub-Signature-256 ──────────────

func TestInboundWebhook_Instagram_ValidSignature(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	handler, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)
	handler.SetInstagramAppSecret("global-ig-secret")

	channelRepo.On("GetByWebhookToken", mock.Anything, "ig-token-1").
		Return(igChannel(strp(testAgentID), json.RawMessage(`{"page_id":"17841400000000001","access_token":"IGQVJ..."}`)), nil)
	msgRepo.On("GetByExternalID", mock.Anything, "ch-ig-1", igMID).
		Return((*domain.Message)(nil), nil)
	// Bot paused → persistence only, no AI goroutine to race with.
	convRepo.On("FindByContact", mock.Anything, "ch-ig-1", igSenderID).Return(&domain.Conversation{
		ID:          "conv-ig-sig",
		AccountID:   testAccountID,
		ChannelID:   "ch-ig-1",
		ContactID:   igSenderID,
		Status:      domain.ConversationOpen,
		IsBotPaused: true,
	}, nil)
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-ig-sig", testAccountID).Return(nil)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Message")).Return(true, nil)

	body := igWebhookBody()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/ig-token-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", hmacHeader(body, "global-ig-secret"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "processed", resp["status"])
	msgRepo.AssertExpectations(t)
}

func TestInboundWebhook_Instagram_InvalidSignature(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	handler, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)
	handler.SetInstagramAppSecret("global-ig-secret")

	channelRepo.On("GetByWebhookToken", mock.Anything, "ig-token-1").
		Return(igChannel(strp(testAgentID), json.RawMessage(`{"page_id":"17841400000000001","access_token":"IGQVJ..."}`)), nil)

	body := igWebhookBody()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/ig-token-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", hmacHeader(body, "attacker-secret"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_SIGNATURE", resp["code"])

	// Forged payload never reaches persistence.
	convRepo.AssertNotCalled(t, "FindByContact", mock.Anything, mock.Anything, mock.Anything)
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestInboundWebhook_Instagram_MissingSignatureRejectedWhenSecretSet(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	handler, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)
	handler.SetInstagramAppSecret("global-ig-secret")

	channelRepo.On("GetByWebhookToken", mock.Anything, "ig-token-1").
		Return(igChannel(strp(testAgentID), json.RawMessage(`{"page_id":"17841400000000001","access_token":"IGQVJ..."}`)), nil)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/ig-token-1", bytes.NewReader(igWebhookBody()))
	req.Header.Set("Content-Type", "application/json")
	// no X-Hub-Signature-256 header at all
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestInboundWebhook_Instagram_PerChannelSecretOverridesGlobal(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	handler, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)
	handler.SetInstagramAppSecret("global-ig-secret")

	// Tenant on their OWN Meta app: channel config carries app_secret and
	// Meta signs with it — the global secret must NOT be used.
	channelRepo.On("GetByWebhookToken", mock.Anything, "ig-token-1").
		Return(igChannel(strp(testAgentID), json.RawMessage(
			`{"page_id":"17841400000000001","access_token":"IGQVJ...","app_secret":"tenant-app-secret"}`)), nil)
	msgRepo.On("GetByExternalID", mock.Anything, "ch-ig-1", igMID).
		Return((*domain.Message)(nil), nil)
	convRepo.On("FindByContact", mock.Anything, "ch-ig-1", igSenderID).Return(&domain.Conversation{
		ID:          "conv-ig-own",
		AccountID:   testAccountID,
		ChannelID:   "ch-ig-1",
		ContactID:   igSenderID,
		Status:      domain.ConversationOpen,
		IsBotPaused: true,
	}, nil)
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-ig-own", testAccountID).Return(nil)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Message")).Return(true, nil)

	body := igWebhookBody()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/ig-token-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", hmacHeader(body, "tenant-app-secret"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	msgRepo.AssertExpectations(t)
}

// ────────────── IG Inbound — echo suppression (no loop) ──────────────

func TestInboundWebhook_Instagram_EchoIgnored(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channelRepo.On("GetByWebhookToken", mock.Anything, "ig-token-1").
		Return(igChannel(strp(testAgentID), igSignedConfig()), nil)

	// Our own bot reply reflected back by Meta — must not re-enter the
	// pipeline (that would be an infinite bot-answers-itself loop).
	echo := []byte(`{
		"object": "instagram",
		"entry": [{
			"id": "17841400000000001",
			"messaging": [{
				"sender": {"id": "17841400000000001"},
				"recipient": {"id": "` + igSenderID + `"},
				"message": {"mid": "echo-mid-1", "text": "Temos sim!", "is_echo": true}
			}]
		}]
	}`)

	w := postIG(router, echo, igTestSecret)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "ignored", resp["status"])

	convRepo.AssertNotCalled(t, "FindByContact", mock.Anything, mock.Anything, mock.Anything)
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// ────────────── IG Inbound — signature fail-CLOSED (US-011) ──────────────

func TestInboundWebhook_Instagram_FailClosedWithoutSecret(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	// NO global secret, NO per-channel app_secret: the webhook must be
	// rejected (same fail-closed policy as SMS/WhatsApp), never
	// WARN+ingest — an unvalidated endpoint accepts forged DMs from
	// anyone who learns the channel token.
	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channelRepo.On("GetByWebhookToken", mock.Anything, "ig-token-1").
		Return(igChannel(strp(testAgentID), json.RawMessage(`{"page_id":"17841400000000001","access_token":"IGQVJ..."}`)), nil)

	w := postIG(router, igWebhookBody(), "")

	assert.Equal(t, http.StatusForbidden, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_SIGNATURE", resp["code"])
	convRepo.AssertNotCalled(t, "FindByContact", mock.Anything, mock.Anything, mock.Anything)
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// ────────────── IG Inbound — batch: every entry[]/messaging[] processed ──────────────

func TestInboundWebhook_Instagram_BatchAllMessagesProcessed(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channelRepo.On("GetByWebhookToken", mock.Anything, "ig-token-1").
		Return(igChannel(nil, igSignedConfig()), nil)

	// Meta batches: two entries, the second with two messaging items —
	// and retries re-deliver the WHOLE batch. Processing only
	// messages[0] silently dropped mid-2 and mid-3.
	const sender2 = "9990001112223334"
	batch := []byte(`{
		"object": "instagram",
		"entry": [
			{
				"id": "17841400000000001",
				"messaging": [{
					"sender": {"id": "` + igSenderID + `"},
					"recipient": {"id": "17841400000000001"},
					"message": {"mid": "batch-mid-1", "text": "primeira"}
				}]
			},
			{
				"id": "17841400000000001",
				"messaging": [
					{
						"sender": {"id": "` + igSenderID + `"},
						"recipient": {"id": "17841400000000001"},
						"message": {"mid": "batch-mid-2", "text": "segunda"}
					},
					{
						"sender": {"id": "` + sender2 + `"},
						"recipient": {"id": "17841400000000001"},
						"message": {"mid": "batch-mid-3", "text": "de outro cliente"}
					}
				]
			}
		]
	}`)

	for _, mid := range []string{"batch-mid-1", "batch-mid-2", "batch-mid-3"} {
		msgRepo.On("GetByExternalID", mock.Anything, "ch-ig-1", mid).
			Return((*domain.Message)(nil), nil)
	}
	pausedConv := func(id, contact string) *domain.Conversation {
		return &domain.Conversation{
			ID: id, AccountID: testAccountID, ChannelID: "ch-ig-1",
			ContactID: contact, Status: domain.ConversationOpen, IsBotPaused: true,
		}
	}
	convRepo.On("FindByContact", mock.Anything, "ch-ig-1", igSenderID).
		Return(pausedConv("conv-batch-1", igSenderID), nil)
	convRepo.On("FindByContact", mock.Anything, "ch-ig-1", sender2).
		Return(pausedConv("conv-batch-2", sender2), nil)
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, mock.Anything, testAccountID).Return(nil)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	for mid, text := range map[string]string{
		"batch-mid-1": "primeira",
		"batch-mid-2": "segunda",
		"batch-mid-3": "de outro cliente",
	} {
		mid, text := mid, text
		msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
			return m.ExternalMessageID != nil && *m.ExternalMessageID == mid &&
				m.Content != nil && *m.Content == text
		})).Return(true, nil).Once()
	}

	w := postIG(router, batch, igTestSecret)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "processed", resp["status"])
	msgRepo.AssertExpectations(t)
	msgRepo.AssertNumberOfCalls(t, "Create", 3)
}

// ────────────── IG Inbound — sender display name resolved (not raw IGSID) ──────────────

func TestInboundWebhook_Instagram_SenderNameResolved(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	// Fake Graph API serving the IGSID profile lookup.
	profileCalls := 0
	graph := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		profileCalls++
		assert.Equal(t, "/"+igSenderID, r.URL.Path)
		assert.Equal(t, "name,username", r.URL.Query().Get("fields"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"Maria Silva","username":"maria.silva"}`))
	}))
	defer graph.Close()

	inboundUC := usecases.NewInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)
	hub := NewHub()
	go hub.Run()
	ig := igadapter.NewInstagramAdapter().WithBaseURL(graph.URL)
	handler := NewInboundWebhookHandler(inboundUC, hub, channelRepo, nil, nil, ig, nil)
	router := setupTestRouter()
	router.POST("/webhooks/:channel_token", handler.HandleInboundWebhook)

	channelRepo.On("GetByWebhookToken", mock.Anything, "ig-token-1").
		Return(igChannel(nil, igSignedConfig()), nil)
	msgRepo.On("GetByExternalID", mock.Anything, "ch-ig-1", igMID).
		Return((*domain.Message)(nil), nil)
	convRepo.On("FindByContact", mock.Anything, "ch-ig-1", igSenderID).Return(&domain.Conversation{
		ID: "conv-ig-name", AccountID: testAccountID, ChannelID: "ch-ig-1",
		ContactID: igSenderID, Status: domain.ConversationOpen, IsBotPaused: true,
	}, nil)
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-ig-name", testAccountID).Return(nil)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
		return m.SenderName != nil && *m.SenderName == "Maria Silva"
	})).Return(true, nil)

	w := postIG(router, igWebhookBody(), igTestSecret)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, profileCalls)
	msgRepo.AssertExpectations(t)
}

// ────────────── Meta GET verification (hub.challenge) ──────────────

func setupVerifyRouter(channelRepo *MockChannelRepo) *routerWithVerify {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)
	handler, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)
	router.GET("/webhooks/:channel_token", handler.HandleWebhookVerify)
	return &routerWithVerify{router: router}
}

type routerWithVerify struct{ router http.Handler }

func (r *routerWithVerify) get(t *testing.T, url string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	r.router.ServeHTTP(w, req)
	return w
}

func TestInstagramWebhookVerify_Success(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	channelRepo.On("GetByWebhookToken", mock.Anything, "ig-token-1").
		Return(igChannel(nil, nil), nil)

	rt := setupVerifyRouter(channelRepo)
	w := rt.get(t, "/webhooks/ig-token-1?hub.mode=subscribe&hub.challenge=1158201444&hub.verify_token=ig-token-1")

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "1158201444", w.Body.String())
}

func TestInstagramWebhookVerify_WrongToken(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	channelRepo.On("GetByWebhookToken", mock.Anything, "ig-token-1").
		Return(igChannel(nil, nil), nil)

	rt := setupVerifyRouter(channelRepo)
	w := rt.get(t, "/webhooks/ig-token-1?hub.mode=subscribe&hub.challenge=1158201444&hub.verify_token=wrong")

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.NotContains(t, w.Body.String(), "1158201444")
}

func TestInstagramWebhookVerify_WrongMode(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	channelRepo.On("GetByWebhookToken", mock.Anything, "ig-token-1").
		Return(igChannel(nil, nil), nil)

	rt := setupVerifyRouter(channelRepo)
	w := rt.get(t, "/webhooks/ig-token-1?hub.mode=unsubscribe&hub.challenge=1158201444&hub.verify_token=ig-token-1")

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestInstagramWebhookVerify_UnknownChannel(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	channelRepo.On("GetByWebhookToken", mock.Anything, "nope").Return(nil, nil)

	rt := setupVerifyRouter(channelRepo)
	w := rt.get(t, "/webhooks/nope?hub.mode=subscribe&hub.challenge=1158201444&hub.verify_token=nope")

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.NotContains(t, w.Body.String(), "1158201444")
}
