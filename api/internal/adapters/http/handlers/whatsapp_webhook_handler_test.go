package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	waadapter "github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	testAppSecret   = "test-app-secret"
	testVerifyToken = "test-verify-token"
)

// computeHMAC computes the HMAC-SHA256 signature in the format Meta sends.
func computeHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// buildWhatsAppPayload creates a realistic WhatsApp Cloud API webhook payload.
func buildWhatsAppPayload(phoneNumberID, from, senderName, text string) []byte {
	payload := map[string]interface{}{
		"object": "whatsapp_business_account",
		"entry": []map[string]interface{}{
			{
				"id": "123456",
				"changes": []map[string]interface{}{
					{
						"value": map[string]interface{}{
							"messaging_product": "whatsapp",
							"metadata": map[string]interface{}{
								"display_phone_number": "15551234567",
								"phone_number_id":      phoneNumberID,
							},
							"contacts": []map[string]interface{}{
								{
									"profile": map[string]interface{}{"name": senderName},
									"wa_id":   from,
								},
							},
							"messages": []map[string]interface{}{
								{
									"from":      from,
									"id":        "wamid.abc123",
									"timestamp": "1234567890",
									"type":      "text",
									"text":      map[string]string{"body": text},
								},
							},
						},
						"field": "messages",
					},
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// buildWhatsAppStatusPayload creates a status update payload (delivered/read).
func buildWhatsAppStatusPayload() []byte {
	payload := map[string]interface{}{
		"object": "whatsapp_business_account",
		"entry": []map[string]interface{}{
			{
				"id": "123456",
				"changes": []map[string]interface{}{
					{
						"value": map[string]interface{}{
							"messaging_product": "whatsapp",
							"metadata": map[string]interface{}{
								"display_phone_number": "15551234567",
								"phone_number_id":      "pn-1",
							},
							"statuses": []map[string]interface{}{
								{
									"id":     "wamid.xyz789",
									"status": "delivered",
								},
							},
						},
						"field": "messages",
					},
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

func setupWhatsAppWebhookHandler(channelRepo *MockChannelRepo) (*WhatsAppWebhookHandler, *gin.Engine) {
	waAdapter := waadapter.NewWhatsAppAdapter("")

	// Create a minimal InboundWebhookUseCase — we mock at repo level
	inboundUC := usecases.NewInboundWebhookUseCase(
		channelRepo,
		new(MockConversationRepo),
		new(MockMessageRepo),
		nil, // processMsg not needed for these tests
		new(MockAgentRepo),
	)

	hub := NewHub()
	go hub.Run()

	handler := NewWhatsAppWebhookHandler(
		inboundUC,
		hub,
		channelRepo,
		nil, // campaignRepo not exercised in this test
		waAdapter,
		testAppSecret,
		testVerifyToken,
	)

	router := setupTestRouter()
	router.GET("/webhooks/whatsapp", handler.HandleVerify)
	router.POST("/webhooks/whatsapp", handler.HandleInbound)

	return handler, router
}

// ────────────── HandleVerify Tests ──────────────

func TestWhatsAppVerify_Success(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	_, router := setupWhatsAppWebhookHandler(channelRepo)

	challenge := "test-challenge-12345"
	url := "/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=" + testVerifyToken + "&hub.challenge=" + challenge

	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, challenge, w.Body.String())
}

func TestWhatsAppVerify_WrongToken(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	_, router := setupWhatsAppWebhookHandler(channelRepo)

	url := "/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=wrong-token&hub.challenge=challenge123"

	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "Forbidden", w.Body.String())
}

func TestWhatsAppVerify_WrongMode(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	_, router := setupWhatsAppWebhookHandler(channelRepo)

	url := "/webhooks/whatsapp?hub.mode=unsubscribe&hub.verify_token=" + testVerifyToken + "&hub.challenge=challenge123"

	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestWhatsAppVerify_MissingParams(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	_, router := setupWhatsAppWebhookHandler(channelRepo)

	req := httptest.NewRequest(http.MethodGet, "/webhooks/whatsapp", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ────────────── HMAC Signature Validation Tests ──────────────

func TestWhatsAppInbound_ValidSignature(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)
	waAdapter := waadapter.NewWhatsAppAdapter("")

	inboundUC := usecases.NewInboundWebhookUseCase(
		channelRepo, convRepo, msgRepo, nil, agentRepo,
	)

	hub := NewHub()
	go hub.Run()

	handler := NewWhatsAppWebhookHandler(
		inboundUC, hub, channelRepo, nil, waAdapter,
		testAppSecret, testVerifyToken,
	)

	router := setupTestRouter()
	router.POST("/webhooks/whatsapp", handler.HandleInbound)

	body := buildWhatsAppPayload("pn-1", "5511999999999", "John", "Hello!")
	sig := computeHMAC(body, testAppSecret)

	channelRepo.On("GetByPhoneNumberID", mock.Anything, "pn-1").Return(&domain.Channel{
		AccountID:    testAccountID,
		ID:           "ch-1",
		AgentID:      strp(testAgentID),
		Type:         "whatsapp",
		WebhookToken: "wh-token",
		IsActive:     true,
	}, nil)

	convRepo.On("FindByContact", mock.Anything, "ch-1", "5511999999999").Return(&domain.Conversation{
		ID:          "conv-sig",
		AccountID:   testAccountID,
		ChannelID:   "ch-1",
		ContactID:   "5511999999999",
		Status:      domain.ConversationOpen,
		IsBotPaused: true,
	}, nil)
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-sig", testAccountID).Return(nil)
	msgRepo.On("GetByExternalID", mock.Anything, "ch-1", mock.Anything).Return(nil, nil)
	msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Message")).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT be 403 (signature is valid), should process successfully
	assert.NotEqual(t, http.StatusForbidden, w.Code)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWhatsAppInbound_InvalidSignature(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	_, router := setupWhatsAppWebhookHandler(channelRepo)

	body := buildWhatsAppPayload("pn-1", "5511999999999", "John", "Hello!")

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256=invalidhash000000000000000000000000000000000000000000000000000000")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestWhatsAppInbound_MissingSignature(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	_, router := setupWhatsAppWebhookHandler(channelRepo)

	body := buildWhatsAppPayload("pn-1", "5511999999999", "John", "Hello!")

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No X-Hub-Signature-256 header
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestWhatsAppInbound_MalformedSignaturePrefix(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	_, router := setupWhatsAppWebhookHandler(channelRepo)

	body := buildWhatsAppPayload("pn-1", "5511999999999", "John", "Hello!")

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "md5=somehash")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestWhatsAppInbound_ForgedSignature_NoFanout proves the HMAC check
// actually stops forged payloads at the door: a body signed with the
// wrong secret is rejected before any channel lookup, conversation
// creation, or message persistence — i.e. an attacker cannot forge an
// inbound message and trigger an AI run on behalf of any tenant.
func TestWhatsAppInbound_ForgedSignature_NoFanout(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)
	waAdapter := waadapter.NewWhatsAppAdapter("")

	inboundUC := usecases.NewInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)
	hub := NewHub()
	go hub.Run()
	handler := NewWhatsAppWebhookHandler(
		inboundUC, hub, channelRepo, nil, waAdapter,
		testAppSecret, testVerifyToken,
	)

	router := setupTestRouter()
	router.POST("/webhooks/whatsapp", handler.HandleInbound)

	body := buildWhatsAppPayload("pn-1", "5511999999999", "John", "Hello!")
	// Signed with the WRONG secret — a valid-looking sha256= signature
	// that simply does not match the configured app secret.
	forged := computeHMAC(body, "attacker-secret")

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", forged)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	// Zero side effects: no channel resolved, no message stored.
	channelRepo.AssertNotCalled(t, "GetByPhoneNumberID")
	msgRepo.AssertNotCalled(t, "Create")
}

// ────────────── App Secret Not Configured ──────────────

func TestWhatsAppInbound_AppSecretNotConfigured(t *testing.T) {
	waAdapter := waadapter.NewWhatsAppAdapter("")

	handler := NewWhatsAppWebhookHandler(
		nil, nil, nil, nil, waAdapter,
		"", // empty app secret
		testVerifyToken,
	)

	router := setupTestRouter()
	router.POST("/webhooks/whatsapp", handler.HandleInbound)

	body := buildWhatsAppPayload("pn-1", "5511999999999", "John", "Hi")
	sig := computeHMAC(body, "anything")

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "webhook secret not configured", resp["error"])
}

// ────────────── Status Update (Ignored) ──────────────

// setupWhatsAppWebhookHandlerWithCampaign mirrors setupWhatsAppWebhookHandler
// but wires a real campaign repo mock so we can exercise applyCampaignStatus.
func setupWhatsAppWebhookHandlerWithCampaign(channelRepo *MockChannelRepo, campaignRepo *MockCampaignRepo) (*WhatsAppWebhookHandler, *gin.Engine) {
	waAdapter := waadapter.NewWhatsAppAdapter("")
	hub := NewHub()
	go hub.Run()
	handler := NewWhatsAppWebhookHandler(
		nil, hub, channelRepo, campaignRepo, waAdapter,
		testAppSecret, testVerifyToken,
	)
	router := setupTestRouter()
	router.POST("/webhooks/whatsapp", handler.HandleInbound)
	return handler, router
}

func TestWhatsAppInbound_StatusUpdate_CorrelatesWithCampaign(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	campaignRepo := new(MockCampaignRepo)
	_, router := setupWhatsAppWebhookHandlerWithCampaign(channelRepo, campaignRepo)

	channelRepo.On("GetByPhoneNumberID", mock.Anything, "pn-1").Return(&domain.Channel{
		ID: "ch-1", AccountID: "acc-1",
	}, nil)
	// The status payload's wamid → campaign lookup hits, status update + counter increment fire.
	camp := &domain.Campaign{ID: "camp-99", AccountID: "acc-1"}
	campaignRepo.On("GetCampaignByProviderMsgID", mock.Anything, "wamid.xyz789").Return(camp, nil)
	campaignRepo.On("UpdateRecipientStatusByProviderID", mock.Anything, "wamid.xyz789", domain.RecipientDelivered).Return(nil)
	campaignRepo.On("IncrementCounter", mock.Anything, "camp-99", domain.RecipientDelivered).Return(nil)

	body := buildWhatsAppStatusPayload()
	sig := computeHMAC(body, testAppSecret)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	campaignRepo.AssertExpectations(t)
}

func TestWhatsAppInbound_StatusUpdate_NotACampaign_NoOp(t *testing.T) {
	// wamid does not belong to any campaign — handler must NOT call update.
	channelRepo := new(MockChannelRepo)
	campaignRepo := new(MockCampaignRepo)
	_, router := setupWhatsAppWebhookHandlerWithCampaign(channelRepo, campaignRepo)

	channelRepo.On("GetByPhoneNumberID", mock.Anything, "pn-1").Return(&domain.Channel{
		ID: "ch-1", AccountID: "acc-1",
	}, nil)
	campaignRepo.On("GetCampaignByProviderMsgID", mock.Anything, "wamid.xyz789").Return(nil, nil)

	body := buildWhatsAppStatusPayload()
	sig := computeHMAC(body, testAppSecret)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	campaignRepo.AssertNotCalled(t, "UpdateRecipientStatusByProviderID")
	campaignRepo.AssertNotCalled(t, "IncrementCounter")
}

func TestWhatsAppInbound_StatusUpdate_Processed(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	_, router := setupWhatsAppWebhookHandler(channelRepo)

	// Status updates now trigger a channel lookup for WebSocket broadcast
	channelRepo.On("GetByPhoneNumberID", mock.Anything, "pn-1").Return(&domain.Channel{
		ID: "ch-1", AccountID: "acc-1",
	}, nil)

	body := buildWhatsAppStatusPayload()
	sig := computeHMAC(body, testAppSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "statuses_processed", resp["status"])
	channelRepo.AssertExpectations(t)
}

// ────────────── Channel Not Found ──────────────

func TestWhatsAppInbound_ChannelNotFound(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	_, router := setupWhatsAppWebhookHandler(channelRepo)

	body := buildWhatsAppPayload("pn-unknown", "5511999999999", "John", "Hello!")
	sig := computeHMAC(body, testAppSecret)

	channelRepo.On("GetByPhoneNumberID", mock.Anything, "pn-unknown").Return(nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code) // always 200 to Meta

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "ignored", resp["status"])

	channelRepo.AssertExpectations(t)
}

// ────────────── Full Inbound Message Flow ──────────────

func TestWhatsAppInbound_FullFlow_Success(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	waAdapter := waadapter.NewWhatsAppAdapter("")

	inboundUC := usecases.NewInboundWebhookUseCase(
		channelRepo, convRepo, msgRepo, nil, agentRepo,
	)

	hub := NewHub()
	go hub.Run()

	handler := NewWhatsAppWebhookHandler(
		inboundUC, hub, channelRepo, nil, waAdapter,
		testAppSecret, testVerifyToken,
	)

	router := setupTestRouter()
	router.POST("/webhooks/whatsapp", handler.HandleInbound)

	body := buildWhatsAppPayload("pn-1", "5511999999999", "John Doe", "Hello bot!")
	sig := computeHMAC(body, testAppSecret)

	channel := &domain.Channel{
		AccountID:    testAccountID,
		ID:           "ch-1",
		AgentID:      strp(testAgentID),
		Type:         "whatsapp",
		WebhookToken: "wh-token",
		IsActive:     true,
	}
	channelRepo.On("GetByPhoneNumberID", mock.Anything, "pn-1").Return(channel, nil)

	// Return existing conversation with bot paused to avoid nil ProcessMessageUseCase
	convRepo.On("FindByContact", mock.Anything, "ch-1", "5511999999999").Return(&domain.Conversation{
		ID:          "conv-wa",
		AccountID:   testAccountID,
		ChannelID:   "ch-1",
		ContactID:   "5511999999999",
		Status:      domain.ConversationOpen,
		IsBotPaused: true,
	}, nil)
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-wa", testAccountID).Return(nil)

	msgRepo.On("GetByExternalID", mock.Anything, "ch-1", mock.Anything).Return(nil, nil)
	msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
		return m.Direction == "inbound" && m.From == "5511999999999" && *m.Content == "Hello bot!"
	})).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
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

// ────────────── Outbox Tests ──────────────

// fakeOutbox is the minimum impl of domain.WAWebhookOutbox needed to
// assert the handler's mark-on-outcome behaviour. We only care about
// the terminal status transitions for these tests.
type fakeOutbox struct {
	mu              sync.Mutex
	inserted        []string
	processed       []string
	skipped         []string
	failed          []string
	failedReasons   map[string]string
	pending         []*domain.WAWebhookEvent
	listPendingCall int
}

func newFakeOutbox() *fakeOutbox {
	return &fakeOutbox{failedReasons: map[string]string{}}
}

func (f *fakeOutbox) Insert(_ context.Context, id string, payload []byte, sig string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inserted = append(f.inserted, id)
	f.pending = append(f.pending, &domain.WAWebhookEvent{
		ID: id, Payload: payload, Signature: sig,
	})
	return nil
}
func (f *fakeOutbox) MarkProcessed(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processed = append(f.processed, id)
	return nil
}
func (f *fakeOutbox) MarkSkipped(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.skipped = append(f.skipped, id)
	return nil
}
func (f *fakeOutbox) MarkFailed(_ context.Context, id string, msg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed = append(f.failed, id)
	f.failedReasons[id] = msg
	return nil
}
func (f *fakeOutbox) MarkDead(_ context.Context, _ string) error { return nil }
func (f *fakeOutbox) ListPending(_ context.Context, _ int) ([]*domain.WAWebhookEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listPendingCall++
	return f.pending, nil
}
func (f *fakeOutbox) PruneProcessed(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (f *fakeOutbox) ListByStatus(_ context.Context, _ string, _ int) ([]*domain.WAWebhookEventDigest, error) {
	return nil, nil
}
func (f *fakeOutbox) ResetForRetry(_ context.Context, _ string) error { return nil }

// TestOutboxFlow_HappyPath asserts that an inbound webhook lands in
// the outbox + the row is marked processed once side effects commit.
func TestOutboxFlow_HappyPath(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)
	waAdapter := waadapter.NewWhatsAppAdapter("")

	inboundUC := usecases.NewInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)
	hub := NewHub()
	go hub.Run()
	handler := NewWhatsAppWebhookHandler(
		inboundUC, hub, channelRepo, nil, waAdapter,
		testAppSecret, testVerifyToken,
	)
	outbox := newFakeOutbox()
	handler.SetOutbox(outbox)

	router := setupTestRouter()
	router.POST("/webhooks/whatsapp", handler.HandleInbound)

	channelRepo.On("GetByPhoneNumberID", mock.Anything, "pn-1").Return(&domain.Channel{
		AccountID: testAccountID, ID: "ch-1", AgentID: strp(testAgentID),
		Type: "whatsapp", WebhookToken: "wh", IsActive: true,
	}, nil)
	convRepo.On("FindByContact", mock.Anything, "ch-1", "5511999999999").Return(&domain.Conversation{
		ID: "conv-1", AccountID: testAccountID, ChannelID: "ch-1",
		ContactID: "5511999999999", Status: domain.ConversationOpen, IsBotPaused: true,
	}, nil)
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-1", testAccountID).Return(nil)
	msgRepo.On("GetByExternalID", mock.Anything, "ch-1", mock.Anything).Return(nil, nil)
	msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Message")).Return(nil)

	body := buildWhatsAppPayload("pn-1", "5511999999999", "John", "ola!")
	sig := computeHMAC(body, testAppSecret)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, outbox.inserted, 1, "should insert outbox row")
	assert.Len(t, outbox.processed, 1, "should mark outbox row processed on success")
	assert.Empty(t, outbox.failed)
	assert.Empty(t, outbox.skipped)
}

// TestOutboxFlow_DuplicateSkipped asserts that a duplicate webhook
// (Meta retried after our handler took >20s) is recorded in the outbox
// and marked `skipped` — neither processed (would fan out twice) nor
// failed (the worker would keep retrying).
func TestOutboxFlow_DuplicateSkipped(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)
	waAdapter := waadapter.NewWhatsAppAdapter("")

	inboundUC := usecases.NewInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)
	hub := NewHub()
	go hub.Run()
	handler := NewWhatsAppWebhookHandler(
		inboundUC, hub, channelRepo, nil, waAdapter,
		testAppSecret, testVerifyToken,
	)
	outbox := newFakeOutbox()
	handler.SetOutbox(outbox)

	router := setupTestRouter()
	router.POST("/webhooks/whatsapp", handler.HandleInbound)

	channelRepo.On("GetByPhoneNumberID", mock.Anything, "pn-1").Return(&domain.Channel{
		AccountID: testAccountID, ID: "ch-1", Type: "whatsapp",
		WebhookToken: "wh", IsActive: true,
	}, nil)
	// Dedupe path: GetByExternalID returns a non-nil result, SaveInbound
	// returns ErrDuplicateInbound.
	msgRepo.On("GetByExternalID", mock.Anything, "ch-1", mock.Anything).Return(&domain.Message{ID: "existing"}, nil)

	body := buildWhatsAppPayload("pn-1", "5511999999999", "John", "ola!")
	sig := computeHMAC(body, testAppSecret)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, outbox.inserted, 1)
	assert.Len(t, outbox.skipped, 1, "duplicate webhook must be skipped, not processed nor failed")
	assert.Empty(t, outbox.processed)
	assert.Empty(t, outbox.failed)
}

// TestOutboxFlow_InvalidSignatureSkipsOutbox asserts that requests
// rejected at the HMAC layer never touch the outbox — there's no
// point recording attacker-controlled payloads.
func TestOutboxFlow_InvalidSignatureSkipsOutbox(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)
	waAdapter := waadapter.NewWhatsAppAdapter("")

	inboundUC := usecases.NewInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)
	hub := NewHub()
	go hub.Run()
	handler := NewWhatsAppWebhookHandler(
		inboundUC, hub, channelRepo, nil, waAdapter,
		testAppSecret, testVerifyToken,
	)
	outbox := newFakeOutbox()
	handler.SetOutbox(outbox)

	router := setupTestRouter()
	router.POST("/webhooks/whatsapp", handler.HandleInbound)

	body := buildWhatsAppPayload("pn-1", "5511999999999", "John", "ola!")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Empty(t, outbox.inserted, "must NOT record forged payloads in the outbox")
}
