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

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// igSignature computes the X-Hub-Signature-256 header value for a raw
// Instagram webhook body, mirroring Meta's HMAC-SHA256 scheme so a test
// can post a payload that passes the handler's fail-closed signature gate.
func igSignature(body []byte, appSecret string) string {
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func setupInboundWebhookHandler(
	channelRepo *MockChannelRepo,
	convRepo *MockConversationRepo,
	msgRepo *MockMessageRepo,
	agentRepo *MockAgentRepo,
) (*InboundWebhookHandler, *gin.Engine) {
	inboundUC := usecases.NewInboundWebhookUseCase(
		channelRepo, convRepo, msgRepo, nil, agentRepo,
	)

	hub := NewHub()
	go hub.Run()

	handler := NewInboundWebhookHandler(
		inboundUC, hub, channelRepo, nil, nil, nil, nil, // no WA/TG/IG/SMS adapters needed for basic tests
	)
	// Inbound mail is signature-gated and fails CLOSED, so every email
	// webhook test has to present a real signature — see signEmailWebhook.
	// Set here rather than per test so a new one cannot accidentally assert
	// against the 403 and call it a pass.
	handler.SetEmailWebhookSecret(testEmailWebhookSecret)

	router := setupTestRouter()
	router.POST("/webhooks/:channel_token", handler.HandleInboundWebhook)

	return handler, router
}

// ────────────── Channel Not Found ──────────────

func TestInboundWebhook_ChannelNotFound(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channelRepo.On("GetByWebhookToken", mock.Anything, "invalid-token").Return(nil, nil)

	body := map[string]interface{}{
		"from":    "5511999999999",
		"content": "Hello",
		"type":    "text",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/invalid-token", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "CHANNEL_NOT_FOUND", resp["code"])

	channelRepo.AssertExpectations(t)
}

// ────────────── Invalid JSON Payload ──────────────

func TestInboundWebhook_InvalidJSON(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	// Channel exists and is type "whatsapp" (default path)
	channelRepo.On("GetByWebhookToken", mock.Anything, "wh-token-1").Return(&domain.Channel{
		AccountID:    testAccountID,
		ID:           "ch-1",
		AgentID:      strp(testAgentID),
		Type:         "whatsapp",
		WebhookToken: "wh-token-1",
		IsActive:     true,
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/wh-token-1", bytes.NewReader([]byte("not valid json{")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_PAYLOAD", resp["code"])
}

// ────────────── WhatsApp (Default) Inbound Success ──────────────

func TestInboundWebhook_WhatsAppDefault_Success(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channel := &domain.Channel{
		AccountID:    testAccountID,
		ID:           "ch-1",
		AgentID:      strp(testAgentID),
		Type:         "whatsapp",
		WebhookToken: "wh-token-1",
		IsActive:     true,
	}
	channelRepo.On("GetByWebhookToken", mock.Anything, "wh-token-1").Return(channel, nil)

	// Return existing conversation with bot paused (to avoid nil ProcessMessageUseCase panic)
	convRepo.On("FindByContact", mock.Anything, "ch-1", "5511888888888").Return(&domain.Conversation{
		ID:          "conv-new",
		AccountID:   testAccountID,
		ChannelID:   "ch-1",
		ContactID:   "5511888888888",
		Status:      domain.ConversationOpen,
		IsBotPaused: true,
	}, nil)
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-new", testAccountID).Return(nil)

	msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
		return m.Direction == "inbound" && m.From == "5511888888888" && *m.Content == "Hi there"
	})).Return(nil)

	body := map[string]interface{}{
		"from":    "5511888888888",
		"content": "Hi there",
		"type":    "text",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/wh-token-1", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "processed", resp["status"])
	assert.NotEmpty(t, resp["conversation_id"])

	channelRepo.AssertExpectations(t)
	agentRepo.AssertExpectations(t)
	convRepo.AssertExpectations(t)
	msgRepo.AssertExpectations(t)
}

// ────────────── Default Type is "text" ──────────────

func TestInboundWebhook_DefaultMessageType(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channel := &domain.Channel{
		AccountID:    testAccountID,
		ID:           "ch-1",
		AgentID:      strp(testAgentID),
		Type:         "whatsapp",
		WebhookToken: "wh-token-1",
		IsActive:     true,
	}
	channelRepo.On("GetByWebhookToken", mock.Anything, "wh-token-1").Return(channel, nil)

	convRepo.On("FindByContact", mock.Anything, "ch-1", "5511777777777").Return(&domain.Conversation{
		ID:          "conv-def",
		AccountID:   testAccountID,
		ChannelID:   "ch-1",
		ContactID:   "5511777777777",
		Status:      domain.ConversationOpen,
		IsBotPaused: true,
	}, nil)
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-def", testAccountID).Return(nil)

	// Verify that type defaults to "text" when not specified
	msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
		return m.Type == "text"
	})).Return(nil)

	body := map[string]interface{}{
		"from":    "5511777777777",
		"content": "No type field",
		// "type" omitted
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/wh-token-1", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	msgRepo.AssertExpectations(t)
}

// ────────────── Existing Conversation (Updates SLA) ──────────────

func TestInboundWebhook_ExistingConversation(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channel := &domain.Channel{
		AccountID:    testAccountID,
		ID:           "ch-1",
		AgentID:      strp(testAgentID),
		Type:         "whatsapp",
		WebhookToken: "wh-token-1",
		IsActive:     true,
	}
	channelRepo.On("GetByWebhookToken", mock.Anything, "wh-token-1").Return(channel, nil)

	// Existing conversation found (bot paused to skip AI processing with nil ProcessMessageUseCase)
	existingConv := &domain.Conversation{
		ID:          "conv-existing",
		AccountID:   testAccountID,
		ChannelID:   "ch-1",
		ContactID:   "5511666666666",
		Status:      domain.ConversationOpen,
		IsBotPaused: true,
	}
	convRepo.On("FindByContact", mock.Anything, "ch-1", "5511666666666").Return(existingConv, nil)
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-existing", testAccountID).Return(nil)

	msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Message")).Return(nil)

	body := map[string]interface{}{
		"from":    "5511666666666",
		"content": "Follow-up message",
		"type":    "text",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/wh-token-1", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "conv-existing", resp["conversation_id"])

	convRepo.AssertExpectations(t)
}

// ────────────── Telegram Channel — Non-message Update Ignored ──────────────

func TestInboundWebhook_Telegram_NonMessageUpdate(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channel := &domain.Channel{
		AccountID:    testAccountID,
		ID:           "ch-tg-1",
		AgentID:      strp(testAgentID),
		Type:         "telegram",
		WebhookToken: "tg-token-1",
		IsActive:     true,
	}
	channelRepo.On("GetByWebhookToken", mock.Anything, "tg-token-1").Return(channel, nil)

	// Telegram update without a message (e.g. bot added to group)
	tgPayload := map[string]interface{}{
		"update_id": 123,
		// no "message" field
	}
	bodyBytes, _ := json.Marshal(tgPayload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/tg-token-1", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "tg-token-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "ignored", resp["status"])
}

// ────────────── Telegram Channel — Text Message Success ──────────────

func TestInboundWebhook_Telegram_TextMessage(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channel := &domain.Channel{
		AccountID:    testAccountID,
		ID:           "ch-tg-1",
		AgentID:      strp(testAgentID),
		Type:         "telegram",
		WebhookToken: "tg-token-1",
		IsActive:     true,
	}
	channelRepo.On("GetByWebhookToken", mock.Anything, "tg-token-1").Return(channel, nil)

	// Telegram dedup: SaveInbound now looks up the prior message by
	// (channel_id, external_message_id) before creating the conversation.
	// For a first-time update the lookup returns (nil, nil).
	msgRepo.On("GetByExternalID", mock.Anything, "ch-tg-1", "12345:1").Return((*domain.Message)(nil), nil)

	// chatID will be "12345" (string form of chat.id)
	convRepo.On("FindByContact", mock.Anything, "ch-tg-1", "12345").Return(&domain.Conversation{
		ID:          "conv-tg",
		AccountID:   testAccountID,
		ChannelID:   "ch-tg-1",
		ContactID:   "12345",
		Status:      domain.ConversationOpen,
		IsBotPaused: true,
	}, nil)
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-tg", testAccountID).Return(nil)

	msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
		return m.Direction == "inbound" && m.From == "12345" && *m.Content == "Hello from Telegram"
	})).Return(nil)

	tgPayload := map[string]interface{}{
		"update_id": 123,
		"message": map[string]interface{}{
			"message_id": 1,
			"from": map[string]interface{}{
				"id":         67890,
				"first_name": "Jane",
				"last_name":  "Doe",
				"username":   "janedoe",
			},
			"chat": map[string]interface{}{
				"id":   12345,
				"type": "private",
			},
			"text": "Hello from Telegram",
			// Fresh timestamp — the handler's replay guard rejects
			// updates older than maxTelegramWebhookAgeSec.
			"date": time.Now().Unix(),
		},
	}
	bodyBytes, _ := json.Marshal(tgPayload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/tg-token-1", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "tg-token-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "processed", resp["status"])

	channelRepo.AssertExpectations(t)
	agentRepo.AssertExpectations(t)
	convRepo.AssertExpectations(t)
	msgRepo.AssertExpectations(t)
}

// ────────────── Telegram Channel — Stale Replay Rejected ──────────────

func TestInboundWebhook_Telegram_StaleReplayRejected(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channel := &domain.Channel{
		AccountID:    testAccountID,
		ID:           "ch-tg-1",
		AgentID:      strp(testAgentID),
		Type:         "telegram",
		WebhookToken: "tg-token-1",
		IsActive:     true,
	}
	channelRepo.On("GetByWebhookToken", mock.Anything, "tg-token-1").Return(channel, nil)

	// message.date two hours old — past maxTelegramWebhookAgeSec (1h).
	tgPayload := map[string]interface{}{
		"update_id": 123,
		"message": map[string]interface{}{
			"message_id": 1,
			"from":       map[string]interface{}{"id": 67890, "first_name": "Jane"},
			"chat":       map[string]interface{}{"id": 12345, "type": "private"},
			"text":       "stale replayed message",
			"date":       time.Now().Add(-2 * time.Hour).Unix(),
		},
	}
	bodyBytes, _ := json.Marshal(tgPayload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/tg-token-1", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "tg-token-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Acked with 200 (no probing signal) but the message is dropped:
	// no conversation/message persistence calls are made.
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "ignored", resp["status"])
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	convRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// ────────────── Handler with nil Hub ──────────────

func TestInboundWebhook_NilHub_DoesNotPanic(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	inboundUC := usecases.NewInboundWebhookUseCase(
		channelRepo, convRepo, msgRepo, nil, agentRepo,
	)

	handler := NewInboundWebhookHandler(
		inboundUC, nil, channelRepo, nil, nil, nil, nil, // nil hub
	)

	router := setupTestRouter()
	router.POST("/webhooks/:channel_token", handler.HandleInboundWebhook)

	channel := &domain.Channel{
		AccountID:    testAccountID,
		ID:           "ch-1",
		AgentID:      strp(testAgentID),
		Type:         "whatsapp",
		WebhookToken: "wh-token-1",
		IsActive:     true,
	}
	channelRepo.On("GetByWebhookToken", mock.Anything, "wh-token-1").Return(channel, nil)

	convRepo.On("FindByContact", mock.Anything, "ch-1", "5511555555555").Return(&domain.Conversation{
		ID:          "conv-nil-hub",
		AccountID:   testAccountID,
		ChannelID:   "ch-1",
		ContactID:   "5511555555555",
		Status:      domain.ConversationOpen,
		IsBotPaused: true,
	}, nil)
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-nil-hub", testAccountID).Return(nil)
	msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Message")).Return(nil)

	body := map[string]interface{}{
		"from":    "5511555555555",
		"content": "Test message",
		"type":    "text",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/wh-token-1", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Should not panic even with nil hub
	assert.NotPanics(t, func() {
		router.ServeHTTP(w, req)
	})

	assert.Equal(t, http.StatusOK, w.Code)
}

// ────────────── Inbound durability: 5xx so the provider retries ──────────────

// A total persist failure on Instagram (which natively redelivers on 5xx and
// has no outbox here) must surface a 500 instead of ACKing 200 — a 200 would
// lose the customer's DM forever. Regression for the audit finding
// "inbound webhook persist-failure is ACKed 200 → permanent message loss".
func TestInboundWebhook_Instagram_PersistFailure_Returns5xx(t *testing.T) {
	const appSecret = "ig-app-secret"

	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channel := &domain.Channel{
		AccountID:    testAccountID,
		ID:           "ch-ig",
		Type:         "instagram",
		WebhookToken: "wh-ig",
		IsActive:     true,
		Config:       json.RawMessage(`{"app_secret":"` + appSecret + `"}`),
	}
	channelRepo.On("GetByWebhookToken", mock.Anything, "wh-ig").Return(channel, nil)

	// Not a dedupe hit — force SaveInbound past the message-id lookup.
	msgRepo.On("GetByExternalID", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	// The persistence layer is down: the conversation lookup errors, so
	// SaveInbound fails and nothing is saved (firstOK stays nil).
	convRepo.On("FindByContact", mock.Anything, "ch-ig", "1254459081234567").
		Return(nil, assert.AnError)

	body := []byte(`{"object":"instagram","entry":[{"id":"17841400000000001","time":1,` +
		`"messaging":[{"sender":{"id":"1254459081234567"},"recipient":{"id":"17841400000000001"},` +
		`"timestamp":1,"message":{"mid":"m-1","text":"oi"}}]}]}`)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/wh-ig", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", igSignature(body, appSecret))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INBOUND_PERSIST_FAILED", resp["code"])

	channelRepo.AssertExpectations(t)
}

// Telegram is deliberately excluded from the 5xx escalation: its webhook
// watchdog flips to "unhealthy" on any 5xx, so a persist failure still ACKs
// 200 (the error is logged for ops). Guards the exclusion boundary.
func TestInboundWebhook_Telegram_PersistFailure_Still200(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channel := &domain.Channel{
		AccountID:    testAccountID,
		ID:           "ch-tg",
		Type:         "telegram",
		WebhookToken: "wh-tg",
		IsActive:     true,
	}
	channelRepo.On("GetByWebhookToken", mock.Anything, "wh-tg").Return(channel, nil)
	msgRepo.On("GetByExternalID", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	convRepo.On("FindByContact", mock.Anything, "ch-tg", "12345").Return(nil, assert.AnError)

	// date:0 → the replay-age guard is skipped (absent date), so the
	// message reaches SaveInbound where the mocked lookup error fires.
	body := []byte(`{"update_id":1,"message":{"message_id":10,"date":0,` +
		`"chat":{"id":12345},"from":{"id":12345,"first_name":"T"},"text":"oi"}}`)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/wh-tg", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wh-tg")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	channelRepo.AssertExpectations(t)
}
