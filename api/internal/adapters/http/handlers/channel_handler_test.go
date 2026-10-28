package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// waadapterMockServer spins up an httptest server and points the
// WhatsApp adapter's metaGraphBaseURL at it, so handler tests can stub
// Meta Graph responses without touching the real API. The hook used
// (SetMetaGraphBaseURLForTest) is defined in the whatsapp package and
// is already exercised by the adapter's own unit tests.
func waadapterMockServer(t *testing.T, fn http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(fn)
	orig := whatsapp.GetBaseURLForTesting()
	whatsapp.SetBaseURLForTesting(srv.URL)
	t.Cleanup(func() {
		whatsapp.SetBaseURLForTesting(orig)
		srv.Close()
	})
	return srv
}

func setupChannelTestRouter(channelRepo *MockChannelRepo, agentRepo *MockAgentRepo, jwtMock *MockJWTService) (*ChannelHandler, *gin.Engine) {
	createUC := usecases.NewCreateChannelUseCase(channelRepo, agentRepo, nil, nil)
	listUC := usecases.NewListChannelsUseCase(channelRepo)
	updateUC := usecases.NewUpdateChannelUseCase(channelRepo, agentRepo)
	deleteUC := usecases.NewDeleteChannelUseCase(channelRepo)
	toggleUC := usecases.NewToggleChannelUseCase(channelRepo)

	handler := NewChannelHandler(createUC, listUC, updateUC, deleteUC, toggleUC, channelRepo, "https://app.example.com", nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	channels := router.Group("/v1/channels", authMw, middleware.RequireRole("admin"))
	{
		channels.GET("", handler.HandleListChannels)
		channels.POST("", handler.HandleCreateChannel)
		channels.PUT("/:id", handler.HandleUpdateChannel)
		channels.DELETE("/:id", handler.HandleDeleteChannel)
		channels.PATCH("/:id/toggle", handler.HandleToggleChannel)
		channels.POST("/:id/whatsapp/register", handler.HandleWhatsAppRegisterPhone)
	}

	return handler, router
}

func TestCreateChannel_Success(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	// Agent belongs to this account
	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID:        testAgentID,
		AccountID: testAccountID,
		Name:      "My Bot",
	}, nil)
	// Plan-limit check expects an existing-list lookup; empty slice means
	// the account is within its plan.MaxChannels bound (defaulted to 2).
	channelRepo.On("ListByAccountID", mock.Anything, testAccountID).Return([]*domain.Channel{}, nil)
	channelRepo.On("Create", mock.Anything, mock.MatchedBy(func(ch *domain.Channel) bool {
		return ch.Name == "WhatsApp Main" && ch.Type == "whatsapp" && ch.AgentID != nil && *ch.AgentID == testAgentID
	})).Return(nil)

	_, router := setupChannelTestRouter(channelRepo, agentRepo, jwtMock)

	body := map[string]interface{}{
		"agent_id": testAgentID,
		"type":     "whatsapp",
		"name":     "WhatsApp Main",
		"config":   map[string]string{"phone_number": "+5511999999999"},
	}
	w := performRequest(router, http.MethodPost, "/v1/channels", body, testToken)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp domain.Channel
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "WhatsApp Main", resp.Name)
	assert.Equal(t, "whatsapp", resp.Type)
	assert.NotEmpty(t, resp.WebhookToken)

	channelRepo.AssertExpectations(t)
	agentRepo.AssertExpectations(t)
}

func TestListChannels_NoAgentIDNeeded(t *testing.T) {
	// After the channels-independence refactor, listing channels no longer
	// requires agent_id — it returns all channels for the account derived
	// from the JWT. This test verifies that GET /channels without query
	// params returns 200 (not 400).
	channelRepo := new(MockChannelRepo)
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("ListByAccountID", mock.Anything, testAccountID).Return([]*domain.Channel{
		{ID: "ch-1", AccountID: testAccountID, AgentID: strp(testAgentID), Type: "whatsapp", Name: "WA"},
		{ID: "ch-2", AccountID: testAccountID, AgentID: nil, Type: "telegram", Name: "Human Only"},
	}, nil)

	_, router := setupChannelTestRouter(channelRepo, agentRepo, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/channels", nil, testToken)
	assert.Equal(t, http.StatusOK, w.Code)

	var channels []map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &channels)
	assert.Len(t, channels, 2)
	channelRepo.AssertExpectations(t)
}

// US-003: the Twilio auth_token is a root account credential — it must
// come back masked from every channel read. Fails on the old code,
// whose hand-kept mask list didn't include auth_token.
func TestListChannels_MasksSMSAuthToken(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("ListByAccountID", mock.Anything, testAccountID).Return([]*domain.Channel{
		{
			ID:        "ch-sms",
			AccountID: testAccountID,
			Type:      "sms",
			Name:      "Twilio SMS",
			Config:    json.RawMessage(`{"account_sid":"AC00000000000000000000000000000000","auth_token":"supersecrettwiliotoken","from_number":"+15551234567"}`),
		},
	}, nil)

	_, router := setupChannelTestRouter(channelRepo, agentRepo, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/channels", nil, testToken)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "supersecrettwiliotoken")

	var channels []map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &channels))
	assert.Len(t, channels, 1)
	config, _ := channels[0]["config"].(map[string]interface{})
	assert.Equal(t, "supe"+strings.Repeat("*", len("supersecrettwiliotoken")-4), config["auth_token"])
	// Non-sensitive fields stay readable so the edit dialog can hydrate.
	assert.Equal(t, "+15551234567", config["from_number"])
	assert.Equal(t, "AC00000000000000000000000000000000", config["account_sid"])
	channelRepo.AssertExpectations(t)
}

func TestDeleteChannel_Success(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID:        testAgentID,
		AccountID: testAccountID,
	}, nil)
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(&domain.Channel{
		AccountID: testAccountID,
		ID:        "ch-1",
		AgentID:   strp(testAgentID),
		Type:      "whatsapp",
		Name:      "WA Channel",
	}, nil)
	channelRepo.On("Delete", mock.Anything, "ch-1").Return(nil)

	_, router := setupChannelTestRouter(channelRepo, agentRepo, jwtMock)

	w := performRequest(router, http.MethodDelete, "/v1/channels/ch-1?agent_id="+testAgentID, nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "channel deleted", resp["message"])

	channelRepo.AssertExpectations(t)
}

func TestToggleChannel_Success(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID:        testAgentID,
		AccountID: testAccountID,
	}, nil)
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(&domain.Channel{
		AccountID: testAccountID,
		ID:        "ch-1",
		AgentID:   strp(testAgentID),
		Type:      "whatsapp",
		Name:      "WA Channel",
		IsActive:  true,
	}, nil)
	// Toggle should flip true -> false
	channelRepo.On("Toggle", mock.Anything, "ch-1", false).Return(&domain.Channel{
		AccountID: testAccountID,
		ID:        "ch-1",
		AgentID:   strp(testAgentID),
		Type:      "whatsapp",
		Name:      "WA Channel",
		IsActive:  false,
	}, nil)

	_, router := setupChannelTestRouter(channelRepo, agentRepo, jwtMock)

	w := performRequest(router, http.MethodPatch, "/v1/channels/ch-1/toggle?agent_id="+testAgentID, nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp domain.Channel
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.False(t, resp.IsActive)

	channelRepo.AssertExpectations(t)
}

// --- WhatsApp register phone (migration flow) -------------------------------

func TestWhatsAppRegisterPhone_Success(t *testing.T) {
	// Sanity: handler should pull waba creds from the channel's JSON
	// config, forward the supplied PIN to Meta /register, then
	// re-subscribe the WABA. Stubs the Graph API via waEmbeddedSrv
	// since the handler reaches Meta synchronously.
	srv := waadapterMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/pn-777/register"):
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"pin":"135790"`) {
				t.Errorf("expected PIN to be forwarded, body=%s", body)
			}
			_, _ = w.Write([]byte(`{"success":true}`))
		case strings.HasSuffix(r.URL.Path, "/waba-42/subscribed_apps"):
			// SubscribeApp re-run is best-effort.
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			t.Errorf("unexpected Meta path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	_ = srv

	channelRepo := new(MockChannelRepo)
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("GetByID", mock.Anything, "ch-wa-1").Return(&domain.Channel{
		ID:        "ch-wa-1",
		AccountID: testAccountID,
		Type:      "whatsapp",
		Name:      "WA",
		Config:    []byte(`{"phone_number_id":"pn-777","waba_id":"waba-42","access_token":"tok-abc"}`),
	}, nil)
	// Handler now persists registration_status onto channels.config via
	// MergeCredentials (JSONB `||`) so concurrent token-refresh writes
	// don't get clobbered by the admin path.
	var persistedFields map[string]string
	channelRepo.On("MergeCredentials", mock.Anything, "ch-wa-1", mock.MatchedBy(func(fields map[string]string) bool {
		persistedFields = fields
		return true
	})).Return(nil)

	_, router := setupChannelTestRouter(channelRepo, agentRepo, jwtMock)

	body := map[string]interface{}{"pin": "135790"}
	w := performRequest(router, http.MethodPost, "/v1/channels/ch-wa-1/whatsapp/register", body, testToken)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["ok"])
	assert.Equal(t, true, resp["registered"])
	assert.Equal(t, "registered", persistedFields["registration_status"])
	assert.Equal(t, "subscribed", persistedFields["subscribe_status"])
	channelRepo.AssertExpectations(t)
}

func TestWhatsAppRegisterPhone_MissingCreds(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("GetByID", mock.Anything, "ch-broken").Return(&domain.Channel{
		ID:        "ch-broken",
		AccountID: testAccountID,
		Type:      "whatsapp",
		Name:      "WA broken",
		Config:    []byte(`{"phone_number_id":"pn-1"}`), // missing access_token + waba_id
	}, nil)

	_, router := setupChannelTestRouter(channelRepo, agentRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/channels/ch-broken/whatsapp/register", map[string]any{"pin": "000000"}, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp domain.AppError
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "MISSING_CREDS", resp.Code)
}

func TestWhatsAppRegisterPhone_InvalidPIN(t *testing.T) {
	// Meta returns 400 code=133005 when PIN mismatches. Handler must
	// propagate as a user-actionable 400/422-family error so the UI
	// can render "PIN incorreto" instead of a generic 500.
	waadapterMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Two-step verification pin mismatch","code":133005,"type":"OAuthException"}}`))
	})
	channelRepo := new(MockChannelRepo)
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("GetByID", mock.Anything, "ch-wa-1").Return(&domain.Channel{
		ID:        "ch-wa-1",
		AccountID: testAccountID,
		Type:      "whatsapp",
		Config:    []byte(`{"phone_number_id":"pn-1","waba_id":"waba-1","access_token":"tok"}`),
	}, nil)
	// Handler persists registration_status=pin_required even on failure
	// so the dashboard can branch on the latest classification without
	// re-calling Meta.
	var persistedFields map[string]string
	channelRepo.On("MergeCredentials", mock.Anything, "ch-wa-1", mock.MatchedBy(func(fields map[string]string) bool {
		persistedFields = fields
		return true
	})).Return(nil)

	_, router := setupChannelTestRouter(channelRepo, agentRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/channels/ch-wa-1/whatsapp/register", map[string]any{"pin": "wrong"}, testToken)

	// mapMetaError now propagates Meta's specific Kind() (133005 →
	// WA_PIN_MISMATCH) so the UI can show the "Esqueci o PIN" CTA
	// instead of the generic register-failed copy.
	assert.GreaterOrEqual(t, w.Code, 400)
	var resp domain.AppError
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "WA_PIN_MISMATCH", resp.Code)
	assert.Equal(t, "pin_required", persistedFields["registration_status"])
	assert.Equal(t, "133005", persistedFields["registration_error_code"])
}

func TestWhatsAppRegisterPhone_WrongChannelType(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("GetByID", mock.Anything, "ch-tg").Return(&domain.Channel{
		ID: "ch-tg", AccountID: testAccountID, Type: "telegram",
		Config: []byte(`{"bot_token":"x"}`),
	}, nil)

	_, router := setupChannelTestRouter(channelRepo, agentRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/channels/ch-tg/whatsapp/register", map[string]any{"pin": "000000"}, testToken)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWhatsAppRegisterPhone_ChannelNotOwned(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("GetByID", mock.Anything, "ch-other").Return(&domain.Channel{
		ID: "ch-other", AccountID: "someone-else", Type: "whatsapp",
		Config: []byte(`{"phone_number_id":"p","waba_id":"w","access_token":"t"}`),
	}, nil)

	_, router := setupChannelTestRouter(channelRepo, agentRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/channels/ch-other/whatsapp/register", map[string]any{"pin": "000000"}, testToken)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// The IMAP poller's last outcome rides on the channel list so a revoked
// grant or a changed password reads as "reconnect" instead of silence.
type fakeInboxCursorReader struct {
	cursors map[string]*domain.EmailInboxCursor
}

func (f *fakeInboxCursorReader) GetByChannel(_ context.Context, channelID string) (*domain.EmailInboxCursor, error) {
	return f.cursors[channelID], nil
}

func TestListChannels_ExposesIMAPHealthOnEmailChannels(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("ListByAccountID", mock.Anything, testAccountID).Return([]*domain.Channel{
		{ID: "ch-mail", AccountID: testAccountID, Type: "email", Name: "Gmail",
			Config: json.RawMessage(`{"auth_method":"google_oauth","from_address":"dona@gmail.com","oauth_refresh_token":"r"}`)},
		{ID: "ch-tg", AccountID: testAccountID, Type: "telegram", Name: "Bot"},
	}, nil)

	handler, router := setupChannelTestRouter(channelRepo, agentRepo, jwtMock)
	polled := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	handler.SetEmailInboxCursorReader(&fakeInboxCursorReader{cursors: map[string]*domain.EmailInboxCursor{
		"ch-mail": {ChannelID: "ch-mail", LastError: "[EMAIL_OAUTH_NOT_CONNECTED] revoked", FailureCount: 3, LastPolledAt: &polled},
	}})

	w := performRequest(router, http.MethodGet, "/v1/channels", nil, testToken)
	assert.Equal(t, http.StatusOK, w.Code)

	var channels []map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &channels))
	assert.Len(t, channels, 2)
	assert.Equal(t, float64(3), channels[0]["imap_failure_count"])
	assert.Equal(t, "[EMAIL_OAUTH_NOT_CONNECTED] revoked", channels[0]["imap_last_error"])
	assert.Equal(t, "2026-09-01T08:00:00Z", channels[0]["imap_last_polled_at"])
	// A non-email channel carries none of it.
	_, has := channels[1]["imap_failure_count"]
	assert.False(t, has)
}
