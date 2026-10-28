package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type takeoverProvider struct {
	sent []*domain.Message
}

func (t *takeoverProvider) SendMessage(ctx context.Context, ch *domain.Channel, to string, msg *domain.Message) error {
	t.sent = append(t.sent, msg)
	return nil
}

func (t *takeoverProvider) HandleWebhook(ctx context.Context, ch *domain.Channel, payload []byte) error {
	return nil
}

// setupTakeoverRouter wires the full takeover stack (handler + use case
// + in-memory hub) so tests exercise the same DI chain main.go builds.
func setupTakeoverRouter(
	convRepo *MockConversationRepo,
	msgRepo *MockMessageRepo,
	channelRepo *MockChannelRepo,
	provider domain.MessageProvider,
	jwtMock *MockJWTService,
) *gin.Engine {
	hub := NewHub()
	go hub.Run()
	providers := map[string]domain.MessageProvider{}
	if provider != nil {
		providers["whatsapp"] = provider
	}
	handler := NewConversationHandler(convRepo, msgRepo, channelRepo, nil, providers, hub, nil)
	takeoverUC := usecases.NewConversationTakeoverUseCase(convRepo, msgRepo, channelRepo, nil, providers, hub)
	handler.SetTakeoverUseCase(takeoverUC)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	conversations := router.Group("/v1/conversations")
	conversations.Use(authMw, middleware.RequireRole("admin", "attendant"))
	{
		conversations.POST("/:id/assign", handler.HandleAssumeConversation)
		conversations.POST("/:id/release", handler.HandleReleaseConversation)
		conversations.POST("/:id/send-message", handler.HandleSendHumanMessage)
	}
	return router
}

func TestHandleAssumeConversation(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	baseConv := &domain.Conversation{
		ID:        "conv-1",
		AccountID: testAccountID,
	}
	now := time.Now()
	userIDCopy := testUserID
	convRepo.On("GetByID", mock.Anything, "conv-1", testAccountID).Return(baseConv, nil)
	convRepo.On("AssumeConversation", mock.Anything, "conv-1", testAccountID, testUserID).Return(nil)
	convRepo.On("GetAssignment", mock.Anything, "conv-1", testAccountID).Return(&domain.ConversationAssignment{
		ConversationID: "conv-1",
		AssignedTo:     &userIDCopy,
		AssignedAt:     &now,
		IsBotPaused:    true,
	}, nil)

	router := setupTakeoverRouter(convRepo, msgRepo, channelRepo, nil, jwtMock)

	w := performRequest(router, http.MethodPost, "/v1/conversations/conv-1/assign", nil, testToken)
	assert.Equal(t, http.StatusOK, w.Code)

	var body domain.ConversationAssignment
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.IsBotPaused)
	assert.Equal(t, testUserID, *body.AssignedTo)
}

func TestHandleReleaseConversation(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	baseConv := &domain.Conversation{
		ID:        "conv-1",
		AccountID: testAccountID,
	}
	convRepo.On("GetByID", mock.Anything, "conv-1", testAccountID).Return(baseConv, nil)
	convRepo.On("ReleaseConversation", mock.Anything, "conv-1", testAccountID).Return(nil)
	convRepo.On("GetAssignment", mock.Anything, "conv-1", testAccountID).Return(&domain.ConversationAssignment{
		ConversationID: "conv-1",
		IsBotPaused:    false,
	}, nil)

	router := setupTakeoverRouter(convRepo, msgRepo, channelRepo, nil, jwtMock)

	w := performRequest(router, http.MethodPost, "/v1/conversations/conv-1/release", nil, testToken)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleSendHumanMessage_BlocksWhenNotAssumed(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	recent := time.Now().Add(-1 * time.Minute)
	running := &domain.Conversation{
		ID:                    "conv-1",
		AccountID:             testAccountID,
		ChannelID:             "ch-1",
		ContactID:             "+5511999999999",
		IsBotPaused:           false,
		LastCustomerMessageAt: &recent,
	}
	convRepo.On("GetByID", mock.Anything, "conv-1", testAccountID).Return(running, nil)
	// The gate now looks the channel up first (email skips it entirely);
	// a chat channel keeps the old assume-first behaviour.
	channelRepo.On("GetByID", mock.Anything, "ch-1").
		Return(&domain.Channel{ID: "ch-1", AccountID: testAccountID, Type: "whatsapp"}, nil)

	router := setupTakeoverRouter(convRepo, msgRepo, channelRepo, nil, jwtMock)

	w := performRequest(router, http.MethodPost, "/v1/conversations/conv-1/send-message",
		map[string]string{"content": "oi"}, testToken)
	assert.Equal(t, http.StatusForbidden, w.Code)

	var body domain.AppError
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "CONV_NOT_ASSUMED", body.Code)
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}
