package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setupWebChatRouter wires the public webchat routes for tests.
// We pass a nil InboundWebhookUseCase; tests that exercise the POST path
// need to override this with a stub. Tests that only hit GET/OPTIONS/bootstrap
// don't depend on the use case.
func setupWebChatRouter(channelRepo *MockChannelRepo, convRepo *MockConversationRepo, msgRepo *MockMessageRepo) (*WebChatHandler, *gin.Engine) {
	h := NewWebChatHandler(channelRepo, convRepo, msgRepo, nil, nil)
	router := setupTestRouter()
	g := router.Group("/v1/widget/:channel_token")
	{
		g.OPTIONS("/messages", h.HandlePreflight)
		g.OPTIONS("/bootstrap", h.HandlePreflight)
		g.GET("/bootstrap", h.HandleBootstrap)
		g.GET("/messages", h.HandlePollMessages)
		// POST is exercised separately in dedicated tests because it needs the use case
	}
	return h, router
}

// webchatChannelWithCfg builds a webchat channel with arbitrary config map.
func webchatChannelWithCfg(token string, cfg map[string]interface{}) *domain.Channel {
	raw, _ := json.Marshal(cfg)
	return &domain.Channel{
		ID:           "ch-web",
		AccountID:    testAccountID,
		Type:         "webchat",
		Name:         "Site",
		WebhookToken: token,
		Config:       json.RawMessage(raw),
		IsActive:     true,
	}
}

// performWebchatRequest issues an HTTP request with custom headers (e.g. Origin).
func performWebchatRequest(router *gin.Engine, method, path string, body interface{}, origin string) *httptest.ResponseRecorder {
	var bodyReader = strings.NewReader("")
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = strings.NewReader(string(b))
	}
	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// ──────────────────────────────────────────────────────────────────
// Channel resolution + tenant safety
// ──────────────────────────────────────────────────────────────────

func TestWebchat_UnknownToken_404(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-bad").Return(nil, nil)

	_, router := setupWebChatRouter(channelRepo, convRepo, msgRepo)
	rec := performWebchatRequest(router, http.MethodGet, "/v1/widget/tok-bad/bootstrap", nil, "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWebchat_NonWebchatChannel_400(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-1").Return(&domain.Channel{
		ID: "x", AccountID: testAccountID, Type: "telegram", IsActive: true,
	}, nil)

	_, router := setupWebChatRouter(channelRepo, convRepo, msgRepo)
	rec := performWebchatRequest(router, http.MethodGet, "/v1/widget/tok-1/bootstrap", nil, "")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	assert.Equal(t, "INVALID_CHANNEL_TYPE", body["code"])
}

func TestWebchat_DisabledChannel_403(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	ch := webchatChannelWithCfg("tok-2", nil)
	ch.IsActive = false
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-2").Return(ch, nil)

	_, router := setupWebChatRouter(channelRepo, convRepo, msgRepo)
	rec := performWebchatRequest(router, http.MethodGet, "/v1/widget/tok-2/bootstrap", nil, "")

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// ──────────────────────────────────────────────────────────────────
// CORS by allowed_origins
// ──────────────────────────────────────────────────────────────────

func TestWebchat_CORS_AllowedOrigin_OK(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	ch := webchatChannelWithCfg("tok-3", map[string]interface{}{
		"allowed_origins": []string{"https://example.com"},
	})
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-3").Return(ch, nil)

	_, router := setupWebChatRouter(channelRepo, convRepo, msgRepo)
	rec := performWebchatRequest(router, http.MethodGet, "/v1/widget/tok-3/bootstrap", nil, "https://example.com")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.Header().Get("Vary"), "Origin")
}

func TestWebchat_CORS_DisallowedOrigin_403(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	ch := webchatChannelWithCfg("tok-4", map[string]interface{}{
		"allowed_origins": []string{"https://example.com"},
	})
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-4").Return(ch, nil)

	_, router := setupWebChatRouter(channelRepo, convRepo, msgRepo)
	rec := performWebchatRequest(router, http.MethodGet, "/v1/widget/tok-4/bootstrap", nil, "https://attacker.com")

	assert.Equal(t, http.StatusForbidden, rec.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	assert.Equal(t, "ORIGIN_NOT_ALLOWED", body["code"])
}

func TestWebchat_CORS_NoAllowedConfigured_AllowsAny(t *testing.T) {
	// When allowed_origins is empty (dev mode), wildcard CORS is returned.
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	ch := webchatChannelWithCfg("tok-5", map[string]interface{}{})
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-5").Return(ch, nil)

	_, router := setupWebChatRouter(channelRepo, convRepo, msgRepo)
	rec := performWebchatRequest(router, http.MethodGet, "/v1/widget/tok-5/bootstrap", nil, "https://anywhere.example")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestWebchat_CORS_TrailingSlashIgnored(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	ch := webchatChannelWithCfg("tok-6", map[string]interface{}{
		"allowed_origins": []string{"https://example.com/"}, // configured with slash
	})
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-6").Return(ch, nil)

	_, router := setupWebChatRouter(channelRepo, convRepo, msgRepo)
	rec := performWebchatRequest(router, http.MethodGet, "/v1/widget/tok-6/bootstrap", nil, "https://example.com")

	assert.Equal(t, http.StatusOK, rec.Code)
}

// ──────────────────────────────────────────────────────────────────
// Bootstrap returns visible config (no secrets)
// ──────────────────────────────────────────────────────────────────

func TestWebchat_Bootstrap_ReturnsConfig(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	ch := webchatChannelWithCfg("tok-7", map[string]interface{}{
		"allowed_origins": []string{"https://example.com"},
		"title":           "Atendimento",
		"welcome_message": "Olá!",
		"primary_color":   "#25D366",
	})
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-7").Return(ch, nil)

	_, router := setupWebChatRouter(channelRepo, convRepo, msgRepo)
	rec := performWebchatRequest(router, http.MethodGet, "/v1/widget/tok-7/bootstrap", nil, "https://example.com")

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	assert.Equal(t, "Atendimento", body["title"])
	assert.Equal(t, "Olá!", body["welcome_message"])
	assert.Equal(t, "#25D366", body["primary_color"])
}

// Bootstrap branding badge (US-172): shown on free/trial, removable on paid.

func TestWebchat_Bootstrap_BrandingShownWhenNoResolver(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	ch := webchatChannelWithCfg("tok-b1", map[string]interface{}{"title": "A"})
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-b1").Return(ch, nil)

	_, router := setupWebChatRouter(channelRepo, new(MockConversationRepo), new(MockMessageRepo))
	rec := performWebchatRequest(router, http.MethodGet, "/v1/widget/tok-b1/bootstrap", nil, "")

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	assert.Equal(t, true, body["show_branding"], "no resolver wired → show the badge (free-tier safe default)")
}

func TestWebchat_Bootstrap_BrandingShownOnTrialPlan(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	ch := webchatChannelWithCfg("tok-b2", map[string]interface{}{"title": "A"})
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-b2").Return(ch, nil)
	planID := "plan-trial"
	accountRepo.On("GetByID", mock.Anything, testAccountID).Return(&domain.Account{ID: testAccountID, PlanID: &planID}, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(&domain.Plan{ID: planID, Name: "Trial"}, nil)

	h := NewWebChatHandler(channelRepo, new(MockConversationRepo), new(MockMessageRepo), nil, nil)
	h.SetPlanResolver(accountRepo, planRepo)
	router := setupTestRouter()
	router.GET("/v1/widget/:channel_token/bootstrap", h.HandleBootstrap)
	rec := performWebchatRequest(router, http.MethodGet, "/v1/widget/tok-b2/bootstrap", nil, "")

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	assert.Equal(t, true, body["show_branding"])
}

func TestWebchat_Bootstrap_BrandingRemovedOnPaidPlan(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	ch := webchatChannelWithCfg("tok-b3", map[string]interface{}{"title": "A"})
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-b3").Return(ch, nil)
	planID := "plan-pro"
	accountRepo.On("GetByID", mock.Anything, testAccountID).Return(&domain.Account{ID: testAccountID, PlanID: &planID}, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(&domain.Plan{ID: planID, Name: "Pro"}, nil)

	h := NewWebChatHandler(channelRepo, new(MockConversationRepo), new(MockMessageRepo), nil, nil)
	h.SetPlanResolver(accountRepo, planRepo)
	router := setupTestRouter()
	router.GET("/v1/widget/:channel_token/bootstrap", h.HandleBootstrap)
	rec := performWebchatRequest(router, http.MethodGet, "/v1/widget/tok-b3/bootstrap", nil, "")

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	assert.Equal(t, false, body["show_branding"], "paid plan removes the badge")
}

// ──────────────────────────────────────────────────────────────────
// OPTIONS preflight
// ──────────────────────────────────────────────────────────────────

func TestWebchat_Preflight_OK(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	ch := webchatChannelWithCfg("tok-8", map[string]interface{}{
		"allowed_origins": []string{"https://example.com"},
	})
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-8").Return(ch, nil)

	_, router := setupWebChatRouter(channelRepo, convRepo, msgRepo)
	rec := performWebchatRequest(router, http.MethodOptions, "/v1/widget/tok-8/messages", nil, "https://example.com")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "POST")
}

func TestWebchat_Preflight_DisallowedOrigin_403(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	ch := webchatChannelWithCfg("tok-9", map[string]interface{}{
		"allowed_origins": []string{"https://example.com"},
	})
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-9").Return(ch, nil)

	_, router := setupWebChatRouter(channelRepo, convRepo, msgRepo)
	rec := performWebchatRequest(router, http.MethodOptions, "/v1/widget/tok-9/messages", nil, "https://evil.example")

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// ──────────────────────────────────────────────────────────────────
// Polling — returns messages of the visitor's conversation
// ──────────────────────────────────────────────────────────────────

func TestWebchat_Poll_NoConversationYet_EmptyList(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	ch := webchatChannelWithCfg("tok-10", nil)
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-10").Return(ch, nil)
	convRepo.On("FindByContact", mock.Anything, "ch-web", "visitor-001").Return(nil, nil)

	_, router := setupWebChatRouter(channelRepo, convRepo, msgRepo)
	rec := performWebchatRequest(router, http.MethodGet, "/v1/widget/tok-10/messages?visitor_id=visitor-001", nil, "")

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	msgs, _ := body["messages"].([]interface{})
	assert.Empty(t, msgs)
	assert.NotEmpty(t, body["now"])
}

func TestWebchat_Poll_ReturnsMessages(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	ch := webchatChannelWithCfg("tok-11", nil)
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-11").Return(ch, nil)
	convRepo.On("FindByContact", mock.Anything, "ch-web", "visitor-002").Return(&domain.Conversation{
		ID: "conv-A", AccountID: testAccountID,
	}, nil)
	hello := "Olá!"
	resp := "Como posso ajudar?"
	msgRepo.On("ListByConversationSince", mock.Anything, "conv-A", mock.Anything).Return([]*domain.Message{
		{ID: "m1", Direction: "inbound", Content: &hello, Timestamp: time.Now()},
		{ID: "m2", Direction: "outbound", Content: &resp, Timestamp: time.Now()},
	}, nil)

	_, router := setupWebChatRouter(channelRepo, convRepo, msgRepo)
	rec := performWebchatRequest(router, http.MethodGet, "/v1/widget/tok-11/messages?visitor_id=visitor-002", nil, "")

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	msgs := body["messages"].([]interface{})
	assert.Len(t, msgs, 2)
	first := msgs[0].(map[string]interface{})
	assert.Equal(t, "m1", first["id"])
	assert.Equal(t, "inbound", first["direction"])
	assert.Equal(t, "Olá!", first["text"])
}

func TestWebchat_Poll_InvalidVisitorID_400(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	ch := webchatChannelWithCfg("tok-12", nil)
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-12").Return(ch, nil)

	_, router := setupWebChatRouter(channelRepo, convRepo, msgRepo)
	cases := []struct{ name, encoded string }{
		{"empty", ""},
		{"spaces", "spaces%20here"},
		{"hash", "with%23hash"},
		{"semicolon", "a%3Bb"},
		{"angle_bracket", "%3Cscript%3E"},
		{"too_long", strings.Repeat("a", 129)},
	}
	for _, c := range cases {
		t.Run("vid="+c.name, func(t *testing.T) {
			rec := performWebchatRequest(router, http.MethodGet,
				"/v1/widget/tok-12/messages?visitor_id="+c.encoded, nil, "")
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestWebchat_Poll_InvalidSince_400(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	ch := webchatChannelWithCfg("tok-13", nil)
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-13").Return(ch, nil)

	_, router := setupWebChatRouter(channelRepo, convRepo, msgRepo)
	rec := performWebchatRequest(router, http.MethodGet,
		"/v1/widget/tok-13/messages?visitor_id=visitor-1&since=not-a-date", nil, "")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebchat_Poll_RespectsSinceCursor(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	ch := webchatChannelWithCfg("tok-14", nil)
	channelRepo.On("GetByWebhookToken", mock.Anything, "tok-14").Return(ch, nil)
	convRepo.On("FindByContact", mock.Anything, "ch-web", "v3").Return(&domain.Conversation{ID: "c"}, nil)
	since := "2026-01-01T00:00:00Z"
	parsed, _ := time.Parse(time.RFC3339Nano, since)
	msgRepo.On("ListByConversationSince", mock.Anything, "c", parsed).Return([]*domain.Message{}, nil)

	_, router := setupWebChatRouter(channelRepo, convRepo, msgRepo)
	rec := performWebchatRequest(router, http.MethodGet,
		"/v1/widget/tok-14/messages?visitor_id=v3&since="+since, nil, "")

	assert.Equal(t, http.StatusOK, rec.Code)
	msgRepo.AssertExpectations(t)
}

// ──────────────────────────────────────────────────────────────────
// Visitor ID validation (input fuzzing — security)
// ──────────────────────────────────────────────────────────────────

func TestWebchat_VisitorIDRegex(t *testing.T) {
	cases := []struct {
		id   string
		pass bool
	}{
		{"abc123", true},
		{"visitor-001", true},
		{"v.001", true},
		{"v_001", true},
		{"abcDEF123_-.xyz", true},
		{"", false},
		{"with space", false},
		{"a/b", false},
		{"a;b", false},
		{"<script>", false},
		{strings.Repeat("a", 129), false}, // > 128 chars
		{strings.Repeat("a", 128), true},  // exactly 128 ok
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			got := visitorIDRegex.MatchString(c.id)
			assert.Equal(t, c.pass, got, "id=%q expected=%v got=%v", c.id, c.pass, got)
		})
	}
}

// ──────────────────────────────────────────────────────────────────
// Origin matcher edge cases
// ──────────────────────────────────────────────────────────────────

func TestOriginAllowed(t *testing.T) {
	cases := []struct {
		origin  string
		allowed []string
		want    bool
	}{
		{"https://x.com", []string{"https://x.com"}, true},
		{"https://x.com/", []string{"https://x.com"}, true}, // trailing slash on origin
		{"https://x.com", []string{"https://x.com/"}, true}, // trailing slash on config
		{"HTTPS://X.COM", []string{"https://x.com"}, true},  // case-insensitive
		{"https://x.com", []string{"https://y.com"}, false},
		{"", []string{"https://x.com"}, false},      // empty origin must be rejected
		{"https://attacker.com", []string{}, false}, // no list → reject when called
	}
	for _, c := range cases {
		t.Run(c.origin, func(t *testing.T) {
			got := originAllowed(c.origin, c.allowed)
			assert.Equal(t, c.want, got)
		})
	}
}
