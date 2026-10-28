package handlers

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// US-010: Twilio inbound SMS webhook at the handler level — realistic
// form-encoded posts through /webhooks/:channel_token with
// X-Twilio-Signature validation and TwiML-safe acks.

const (
	smsAuthToken  = "twilio-auth-token-1"
	smsFrom       = "+15558675310"
	smsTo         = "+15550001111"
	smsMessageSID = "SMa2b53e9dcd7a4b3fb1a93a7c4d1f5a10"
)

func smsChannel(agentID *string) *domain.Channel {
	return &domain.Channel{
		AccountID:    testAccountID,
		ID:           "ch-sms-1",
		AgentID:      agentID,
		Type:         "sms",
		WebhookToken: "sms-token-1",
		IsActive:     true,
		Config: json.RawMessage(`{"account_sid":"ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",` +
			`"auth_token":"` + smsAuthToken + `","from_number":"` + smsTo + `"}`),
	}
}

func smsForm(body string) url.Values {
	return url.Values{
		"MessageSid": {smsMessageSID},
		"SmsSid":     {smsMessageSID},
		"AccountSid": {"ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"},
		"From":       {smsFrom},
		"To":         {smsTo},
		"Body":       {body},
		"NumMedia":   {"0"},
		"SmsStatus":  {"received"},
		"ApiVersion": {"2010-04-01"},
	}
}

// twilioSign computes X-Twilio-Signature the way Twilio does for a
// form-encoded POST: HMAC-SHA1 over the full URL plus the form params
// concatenated key+value in sorted key order, base64-encoded.
func twilioSign(authToken, fullURL string, form url.Values) string {
	keys := make([]string, 0, len(form))
	for k := range form {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	payload := fullURL
	for _, k := range keys {
		payload += k + form.Get(k)
	}
	mac := hmac.New(sha1.New, []byte(authToken))
	mac.Write([]byte(payload))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func postTwilio(router http.Handler, form url.Values, signature string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/webhooks/sms-token-1",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if signature != "" {
		req.Header.Set("X-Twilio-Signature", signature)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// signedURL is what the handler reconstructs for an httptest request:
// plain HTTP, host "example.com", no query string.
const smsSignedURL = "http://example.com/webhooks/sms-token-1"

// ────────────── Inbound SMS — conversation + message + AI invoked ──────────────

func TestInboundWebhook_SMS_AIPipelineInvoked(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channelRepo.On("GetByWebhookToken", mock.Anything, "sms-token-1").
		Return(smsChannel(strp(testAgentID)), nil)
	msgRepo.On("GetByExternalID", mock.Anything, "ch-sms-1", smsMessageSID).
		Return((*domain.Message)(nil), nil)

	// First text from this number → conversation created, keyed on the
	// E.164 phone as the sender identity.
	convRepo.On("FindByContact", mock.Anything, "ch-sms-1", smsFrom).Return(nil, nil)
	convRepo.On("Create", mock.Anything, mock.MatchedBy(func(cv *domain.Conversation) bool {
		return cv.AccountID == testAccountID && cv.ChannelID == "ch-sms-1" &&
			cv.ContactID == smsFrom
	})).Return(nil)
	convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
		return m.Direction == "inbound" && m.From == smsFrom &&
			*m.Content == "Are you open on Saturday?" &&
			m.ExternalMessageID != nil && *m.ExternalMessageID == smsMessageSID
	})).Return(true, nil)

	// Agent assigned + bot active → ProcessAI runs on the detached
	// goroutine; its first repo touch is the agent lookup (nil, nil)
	// makes it stop right after — proving the pipeline was entered.
	aiInvoked := make(chan struct{})
	agentRepo.On("GetByID", mock.Anything, testAgentID).
		Run(func(args mock.Arguments) { close(aiInvoked) }).
		Return(nil, nil)

	form := smsForm("Are you open on Saturday?")
	w := postTwilio(router, form, twilioSign(smsAuthToken, smsSignedURL, form))

	assert.Equal(t, http.StatusOK, w.Code)
	// TwiML-safe ack: Twilio parses the body as TwiML, so it must be an
	// XML <Response/>, not the JSON the other channels get.
	assert.Contains(t, w.Header().Get("Content-Type"), "text/xml")
	assert.Contains(t, w.Body.String(), "<Response/>")

	select {
	case <-aiInvoked:
	case <-time.After(2 * time.Second):
		t.Fatal("AI pipeline was not invoked for an agent-assigned SMS channel")
	}

	channelRepo.AssertExpectations(t)
	convRepo.AssertExpectations(t)
	msgRepo.AssertExpectations(t)
}

// ────────────── Inbound SMS — no agent → AI skipped ──────────────

func TestInboundWebhook_SMS_NoAgent_AISkipped(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channelRepo.On("GetByWebhookToken", mock.Anything, "sms-token-1").
		Return(smsChannel(nil), nil)
	msgRepo.On("GetByExternalID", mock.Anything, "ch-sms-1", smsMessageSID).
		Return((*domain.Message)(nil), nil)
	convRepo.On("FindByContact", mock.Anything, "ch-sms-1", smsFrom).Return(nil, nil)
	convRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Conversation")).Return(nil)
	convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Message")).Return(true, nil)

	form := smsForm("Hello")
	w := postTwilio(router, form, twilioSign(smsAuthToken, smsSignedURL, form))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "<Response/>")

	time.Sleep(150 * time.Millisecond)
	agentRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	msgRepo.AssertExpectations(t)
}

// ────────────── X-Twilio-Signature ──────────────

func TestInboundWebhook_SMS_InvalidSignature(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channelRepo.On("GetByWebhookToken", mock.Anything, "sms-token-1").
		Return(smsChannel(strp(testAgentID)), nil)

	form := smsForm("Hello")
	w := postTwilio(router, form, twilioSign("attacker-token", smsSignedURL, form))

	assert.Equal(t, http.StatusForbidden, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_SIGNATURE", resp["code"])

	convRepo.AssertNotCalled(t, "FindByContact", mock.Anything, mock.Anything, mock.Anything)
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestInboundWebhook_SMS_MissingSignature(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channelRepo.On("GetByWebhookToken", mock.Anything, "sms-token-1").
		Return(smsChannel(strp(testAgentID)), nil)

	w := postTwilio(router, smsForm("Hello"), "")

	assert.Equal(t, http.StatusForbidden, w.Code)
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestInboundWebhook_SMS_TamperedBodyRejected(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channelRepo.On("GetByWebhookToken", mock.Anything, "sms-token-1").
		Return(smsChannel(strp(testAgentID)), nil)

	// Signature computed over a different Body than the one posted.
	signed := twilioSign(smsAuthToken, smsSignedURL, smsForm("original text"))
	w := postTwilio(router, smsForm("tampered text"), signed)

	assert.Equal(t, http.StatusForbidden, w.Code)
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestInboundWebhook_SMS_MissingAuthTokenConfigFailsClosed(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	broken := smsChannel(strp(testAgentID))
	broken.Config = json.RawMessage(`{"account_sid":"ACxxx","from_number":"+15550001111"}`)
	channelRepo.On("GetByWebhookToken", mock.Anything, "sms-token-1").
		Return(broken, nil)

	form := smsForm("Hello")
	w := postTwilio(router, form, twilioSign(smsAuthToken, smsSignedURL, form))

	assert.Equal(t, http.StatusForbidden, w.Code)
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// ────────────── Non-message deliveries acked without ingesting ──────────────

func TestInboundWebhook_SMS_StatusCallbackIgnored(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channelRepo.On("GetByWebhookToken", mock.Anything, "sms-token-1").
		Return(smsChannel(strp(testAgentID)), nil)

	// A delivery-status callback mistakenly pointed at the message URL:
	// signed correctly, but not an inbound message.
	form := url.Values{
		"MessageSid":    {smsMessageSID},
		"AccountSid":    {"ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"},
		"From":          {smsTo},
		"To":            {smsFrom},
		"MessageStatus": {"delivered"},
		"SmsStatus":     {"delivered"},
	}
	w := postTwilio(router, form, twilioSign(smsAuthToken, smsSignedURL, form))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/xml")
	assert.Contains(t, w.Body.String(), "<Response/>")

	convRepo.AssertNotCalled(t, "FindByContact", mock.Anything, mock.Anything, mock.Anything)
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// ────────────── Duplicate webhook retry acked with TwiML ──────────────

func TestInboundWebhook_SMS_DuplicateAckedWithTwiML(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channelRepo.On("GetByWebhookToken", mock.Anything, "sms-token-1").
		Return(smsChannel(strp(testAgentID)), nil)
	// Dedupe hit: this MessageSid was already ingested.
	msgRepo.On("GetByExternalID", mock.Anything, "ch-sms-1", smsMessageSID).
		Return(&domain.Message{ID: "msg-already-there"}, nil)

	form := smsForm("Are you open on Saturday?")
	w := postTwilio(router, form, twilioSign(smsAuthToken, smsSignedURL, form))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/xml")
	assert.Contains(t, w.Body.String(), "<Response/>")
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// ────────────── MMS media mapping ──────────────

func TestInboundWebhook_SMS_MMSMediaMapped(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channelRepo.On("GetByWebhookToken", mock.Anything, "sms-token-1").
		Return(smsChannel(nil), nil)
	msgRepo.On("GetByExternalID", mock.Anything, "ch-sms-1", smsMessageSID).
		Return((*domain.Message)(nil), nil)
	convRepo.On("FindByContact", mock.Anything, "ch-sms-1", smsFrom).Return(&domain.Conversation{
		ID:          "conv-sms-1",
		AccountID:   testAccountID,
		ChannelID:   "ch-sms-1",
		ContactID:   smsFrom,
		Status:      domain.ConversationOpen,
		IsBotPaused: true,
	}, nil)
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-sms-1", testAccountID).Return(nil)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
		return m.Type == "image" && m.MediaURL != nil &&
			*m.MediaURL == "https://api.twilio.com/2010-04-01/Accounts/AC/Messages/MM/Media/ME0"
	})).Return(true, nil)

	form := smsForm("")
	form.Set("NumMedia", "1")
	form.Set("MediaUrl0", "https://api.twilio.com/2010-04-01/Accounts/AC/Messages/MM/Media/ME0")
	form.Set("MediaContentType0", "image/jpeg")
	w := postTwilio(router, form, twilioSign(smsAuthToken, smsSignedURL, form))

	assert.Equal(t, http.StatusOK, w.Code)
	msgRepo.AssertExpectations(t)
}

// ────────────── MMS multi-attachment — every media ingested (US-011) ──────────────

func TestInboundWebhook_SMS_MMSAllAttachmentsIngested(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	_, router := setupInboundWebhookHandler(channelRepo, convRepo, msgRepo, agentRepo)

	channelRepo.On("GetByWebhookToken", mock.Anything, "sms-token-1").
		Return(smsChannel(nil), nil)
	// Each attachment row dedupes on its own external id (#n suffix for
	// the extras) so Twilio webhook retries stay idempotent per media.
	msgRepo.On("GetByExternalID", mock.Anything, "ch-sms-1", smsMessageSID).
		Return((*domain.Message)(nil), nil)
	msgRepo.On("GetByExternalID", mock.Anything, "ch-sms-1", smsMessageSID+"#1").
		Return((*domain.Message)(nil), nil)
	convRepo.On("FindByContact", mock.Anything, "ch-sms-1", smsFrom).Return(&domain.Conversation{
		ID:          "conv-sms-mm",
		AccountID:   testAccountID,
		ChannelID:   "ch-sms-1",
		ContactID:   smsFrom,
		Status:      domain.ConversationOpen,
		IsBotPaused: true,
	}, nil)
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-sms-mm", testAccountID).Return(nil)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	// First message carries the text body + first attachment...
	msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
		return m.Type == "image" && m.MediaURL != nil &&
			*m.MediaURL == "https://api.twilio.com/media/ME0" &&
			m.Content != nil && *m.Content == "duas fotos"
	})).Return(true, nil).Once()
	// ...and the SECOND attachment becomes its own media message
	// (MediaUrl0-only parsing silently dropped it).
	msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
		return m.Type == "video" && m.MediaURL != nil &&
			*m.MediaURL == "https://api.twilio.com/media/ME1" &&
			m.ExternalMessageID != nil && *m.ExternalMessageID == smsMessageSID+"#1"
	})).Return(true, nil).Once()

	form := smsForm("duas fotos")
	form.Set("NumMedia", "2")
	form.Set("MediaUrl0", "https://api.twilio.com/media/ME0")
	form.Set("MediaContentType0", "image/jpeg")
	form.Set("MediaUrl1", "https://api.twilio.com/media/ME1")
	form.Set("MediaContentType1", "video/mp4")
	w := postTwilio(router, form, twilioSign(smsAuthToken, smsSignedURL, form))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "<Response/>")
	msgRepo.AssertExpectations(t)
	msgRepo.AssertNumberOfCalls(t, "Create", 2)
}
