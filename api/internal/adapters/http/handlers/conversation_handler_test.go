package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/channels/inapp"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupConversationTestRouter(convRepo *MockConversationRepo, msgRepo *MockMessageRepo, channelRepo *MockChannelRepo, jwtMock *MockJWTService) *gin.Engine {
	handler := NewConversationHandler(convRepo, msgRepo, channelRepo, nil, nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	conversations := router.Group("/v1/conversations")
	conversations.Use(authMw, middleware.RequireRole("admin", "attendant"))
	{
		conversations.GET("", handler.HandleListConversations)
		conversations.GET("/:id/messages", handler.HandleListMessages)
	}
	return router
}

func TestListConversations_Success(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	now := time.Now()
	lastMsg := "Hello there"
	summaries := []*domain.ConversationSummary{
		{
			ID:          "conv-1",
			AccountID:   testAccountID,
			ChannelID:   "ch-1",
			ContactID:   "contact-1",
			ContactName: "Alice",
			Status:      domain.ConversationOpen,
			ChannelType: "whatsapp",
			LastMessage: &lastMsg,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	// Default pagination: limit=50, offset=0, status=open
	convRepo.On("ListEnriched", mock.Anything, testAccountID, "open", int32(50), int32(0), "", "", "").Return(summaries, nil)

	router := setupConversationTestRouter(convRepo, msgRepo, channelRepo, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/conversations", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []domain.ConversationSummary
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "Alice", resp[0].ContactName)
	assert.Equal(t, "whatsapp", resp[0].ChannelType)

	convRepo.AssertExpectations(t)
}

// TestListConversations_NoNPlusOne is the US-060 N+1 regression guard. A page of
// 100 conversations must be served by a SINGLE ListEnriched call — the enriched
// query folds contact/channel/agent/last-message into one row per conversation
// and tags are batch-fetched inside the repo. A per-conversation fetch would
// surface here as N>1 calls (or extra repo methods). See docs/db-perf.md.
func TestListConversations_NoNPlusOne(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	now := time.Now()
	lastMsg := "Hello there"
	summaries := make([]*domain.ConversationSummary, 100)
	for i := range summaries {
		summaries[i] = &domain.ConversationSummary{
			ID:          "conv-" + strconv.Itoa(i),
			AccountID:   testAccountID,
			ChannelID:   "ch-1",
			ContactID:   "contact-" + strconv.Itoa(i),
			ContactName: "Contact " + strconv.Itoa(i),
			Status:      domain.ConversationOpen,
			ChannelType: "whatsapp",
			LastMessage: &lastMsg,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
	}

	convRepo.On("ListEnriched", mock.Anything, testAccountID, "open", int32(50), int32(0), "", "", "").Return(summaries, nil)

	router := setupConversationTestRouter(convRepo, msgRepo, channelRepo, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/conversations", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []domain.ConversationSummary
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 100)

	// The crux of the N+1 guard: exactly one enriched query for the whole page.
	convRepo.AssertNumberOfCalls(t, "ListEnriched", 1)
	convRepo.AssertExpectations(t)
}

func TestListConversations_PaginationClamped(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	// limit=999 should be clamped to 200
	convRepo.On("ListEnriched", mock.Anything, testAccountID, "open", int32(200), int32(0), "", "", "").Return([]*domain.ConversationSummary{}, nil)

	router := setupConversationTestRouter(convRepo, msgRepo, channelRepo, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/conversations?limit=999", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	convRepo.AssertExpectations(t)
}

func TestListConversations_AttendantAllowed(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "attendant") // attendant role

	convRepo.On("ListEnriched", mock.Anything, testAccountID, "open", int32(50), int32(0), "", "", "").Return([]*domain.ConversationSummary{}, nil)

	router := setupConversationTestRouter(convRepo, msgRepo, channelRepo, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/conversations", nil, testToken)

	// Attendants should have access to conversations
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListConversations_InvalidStatusReturns400(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	router := setupConversationTestRouter(convRepo, msgRepo, channelRepo, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/conversations?status=bogus", nil, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp domain.AppError
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_FILTER", resp.Code)

	// Repo must not be hit — validation aborts before any query.
	convRepo.AssertNotCalled(t, "ListEnriched", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestListConversations_InvalidPriorityReturns400(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	router := setupConversationTestRouter(convRepo, msgRepo, channelRepo, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/conversations?priority=critical", nil, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp domain.AppError
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_FILTER", resp.Code)

	convRepo.AssertNotCalled(t, "ListEnriched", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestListConversations_InvalidSortReturns400(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	router := setupConversationTestRouter(convRepo, msgRepo, channelRepo, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/conversations?sort=DROP%20TABLE", nil, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp domain.AppError
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_FILTER", resp.Code)

	convRepo.AssertNotCalled(t, "ListEnriched", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestListConversations_InvalidAssignedReturns400(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	router := setupConversationTestRouter(convRepo, msgRepo, channelRepo, jwtMock)

	// Contains a SQL metacharacter — must be rejected.
	w := performRequest(router, http.MethodGet, "/v1/conversations?assigned=%27%20OR%201%3D1", nil, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp domain.AppError
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_FILTER", resp.Code)

	convRepo.AssertNotCalled(t, "ListEnriched", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestListConversations_ValidPriorityAndSortAccepted(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	convRepo.On("ListEnriched", mock.Anything, testAccountID, "open", int32(50), int32(0), "oldest", "", "").Return([]*domain.ConversationSummary{}, nil)

	router := setupConversationTestRouter(convRepo, msgRepo, channelRepo, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/conversations?priority=high&sort=oldest", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)
	convRepo.AssertExpectations(t)
}

// US-019 (review fixes): the assigned filter must reach the repository
// (SQL WHERE, before LIMIT/OFFSET) instead of being applied in memory
// over an already-paginated page — post-pagination filtering hid an
// attendant's conversations whenever they ranked past the page limit.
func TestListConversations_AssignedFilterPushedToSQL(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	attendant := "01HZZZZZZZZZZZZZZZZZZZZZZA"
	assigned := []*domain.ConversationSummary{
		{
			ID:          "conv-mine",
			AccountID:   testAccountID,
			ChannelID:   "ch-1",
			ContactID:   "contact-1",
			ContactName: "Alice",
			Status:      domain.ConversationOpen,
			AssignedTo:  &attendant,
		},
	}
	convRepo.On("ListEnriched", mock.Anything, testAccountID, "open", int32(50), int32(0), "", attendant, "").Return(assigned, nil)

	router := setupConversationTestRouter(convRepo, msgRepo, channelRepo, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/conversations?assigned="+attendant, nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []domain.ConversationSummary
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	convRepo.AssertExpectations(t)
}

// US-020: `assigned=none` (the "Não atribuídas" bucket) must pass the
// filter whitelist and reach the repo verbatim — the pre-US-020
// whitelist 400'd anything that wasn't a 26-char user id.
func TestListConversations_UnassignedFilterAccepted(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	unassigned := []*domain.ConversationSummary{
		{
			ID:          "conv-nobody",
			AccountID:   testAccountID,
			ChannelID:   "ch-1",
			ContactID:   "contact-1",
			ContactName: "Bob",
			Status:      domain.ConversationOpen,
		},
	}
	convRepo.On("ListEnriched", mock.Anything, testAccountID, "open", int32(50), int32(0), "", domain.AssignedFilterUnassigned, "").Return(unassigned, nil)

	router := setupConversationTestRouter(convRepo, msgRepo, channelRepo, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/conversations?assigned=none", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []domain.ConversationSummary
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	convRepo.AssertExpectations(t)
}

func TestListConversations_CustomStatusAndPagination(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	convRepo.On("ListEnriched", mock.Anything, testAccountID, "closed", int32(10), int32(20), "", "", "").Return([]*domain.ConversationSummary{}, nil)

	router := setupConversationTestRouter(convRepo, msgRepo, channelRepo, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/conversations?status=closed&limit=10&offset=20", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	convRepo.AssertExpectations(t)
}

func TestListMessages_Success(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	now := time.Now()
	convID := "conv-1"
	content := "Hello!"

	// Verify conversation ownership
	convRepo.On("GetByID", mock.Anything, convID, testAccountID).Return(&domain.Conversation{
		ID:        convID,
		AccountID: testAccountID,
		ChannelID: "ch-1",
		ContactID: "contact-1",
		Status:    domain.ConversationOpen,
		CreatedAt: now,
	}, nil)

	msgRepo.On("ListByConversationID", mock.Anything, convID, int32(100), int32(0)).Return([]*domain.Message{
		{
			ID:             "msg-1",
			ChannelID:      "ch-1",
			ConversationID: &convID,
			From:           "customer",
			Type:           "text",
			Content:        &content,
			Direction:      "inbound",
			Timestamp:      now,
		},
	}, nil)

	router := setupConversationTestRouter(convRepo, msgRepo, channelRepo, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/conversations/conv-1/messages", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var msgs []domain.Message
	err := json.Unmarshal(w.Body.Bytes(), &msgs)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "msg-1", msgs[0].ID)

	convRepo.AssertExpectations(t)
	msgRepo.AssertExpectations(t)
}

func TestResumeBot_Success(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	now := time.Now()

	convRepo.On("GetByID", mock.Anything, "conv-1", testAccountID).Return(&domain.Conversation{
		ID:          "conv-1",
		AccountID:   testAccountID,
		ChannelID:   "ch-1",
		ContactID:   "contact-1",
		Status:      domain.ConversationOpen,
		IsBotPaused: true,
		CreatedAt:   now,
	}, nil)
	convRepo.On("SetBotPaused", mock.Anything, "conv-1", testAccountID, false).Return(nil)

	handler := NewConversationHandler(convRepo, msgRepo, channelRepo, nil, nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	conversations := router.Group("/v1/conversations")
	conversations.Use(authMw, middleware.RequireRole("admin", "attendant"))
	{
		conversations.PATCH("/:id/resume-bot", handler.HandleResumeBot)
	}

	w := performRequest(router, http.MethodPatch, "/v1/conversations/conv-1/resume-bot", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "bot_resumed", resp["status"])
	assert.Equal(t, false, resp["bot_paused"])

	convRepo.AssertExpectations(t)
}

func TestHumanReply_Success(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	mockProvider := new(MockMessageProvider)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	now := time.Now()
	recentMsg := now.Add(-1 * time.Hour)

	convRepo.On("GetByID", mock.Anything, "conv-1", testAccountID).Return(&domain.Conversation{
		ID:                    "conv-1",
		AccountID:             testAccountID,
		ChannelID:             "ch-1",
		ContactID:             "contact-1",
		Status:                domain.ConversationOpen,
		LastCustomerMessageAt: &recentMsg,
		CreatedAt:             now,
	}, nil)
	convRepo.On("SetBotPaused", mock.Anything, "conv-1", testAccountID, true).Return(nil)
	convRepo.On("ClearWaitingSince", mock.Anything, "conv-1", testAccountID).Return(nil)
	msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Message")).Return(nil)
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(&domain.Channel{
		AccountID: testAccountID,
		ID:        "ch-1",
		AgentID:   strp("agent-1"),
		Type:      "whatsapp",
		Name:      "WA Channel",
	}, nil)
	mockProvider.On("SendMessage", mock.Anything, mock.Anything, "contact-1", mock.AnythingOfType("*domain.Message")).Return(nil)

	providers := map[string]domain.MessageProvider{
		"whatsapp": mockProvider,
	}

	hub := NewHub()
	go hub.Run()

	handler := NewConversationHandler(convRepo, msgRepo, channelRepo, nil, providers, hub, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	conversations := router.Group("/v1/conversations")
	conversations.Use(authMw, middleware.RequireRole("admin", "attendant"))
	{
		conversations.POST("/:id/reply", handler.HandleHumanReply)
	}

	body := map[string]string{"message": "Hello from human agent"}
	w := performRequest(router, http.MethodPost, "/v1/conversations/conv-1/reply", body, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "reply_sent", resp["status"])
	assert.Equal(t, true, resp["bot_paused"])

	convRepo.AssertExpectations(t)
	msgRepo.AssertExpectations(t)
	channelRepo.AssertExpectations(t)
	mockProvider.AssertExpectations(t)
}

// US-009 (review fixes): a channel-send failure used to be swallowed —
// the API answered 200 "reply_sent" and the attendant never learned the
// customer didn't get the reply. Now the row is stamped failed and the
// response is a MESSAGE_SEND_FAILED error.
func TestHumanReply_SendFailure_NotSilent(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	mockProvider := new(MockMessageProvider)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	now := time.Now()
	recentMsg := now.Add(-1 * time.Hour)

	convRepo.On("GetByID", mock.Anything, "conv-1", testAccountID).Return(&domain.Conversation{
		ID:                    "conv-1",
		AccountID:             testAccountID,
		ChannelID:             "ch-1",
		ContactID:             "contact-1",
		Status:                domain.ConversationOpen,
		LastCustomerMessageAt: &recentMsg,
		CreatedAt:             now,
	}, nil)
	convRepo.On("SetBotPaused", mock.Anything, "conv-1", testAccountID, true).Return(nil)
	convRepo.On("ClearWaitingSince", mock.Anything, "conv-1", testAccountID).Return(nil)
	var persistedID string
	msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
		persistedID = m.ID
		return true
	})).Return(nil)
	msgRepo.On("MarkSendFailed", mock.Anything, mock.AnythingOfType("string"), "twilio: 21610 blocked recipient").Return(nil)
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(&domain.Channel{
		AccountID: testAccountID,
		ID:        "ch-1",
		Type:      "sms",
		Name:      "SMS Channel",
	}, nil)
	mockProvider.On("SendMessage", mock.Anything, mock.Anything, "contact-1", mock.AnythingOfType("*domain.Message")).
		Return(errors.New("twilio: 21610 blocked recipient"))

	providers := map[string]domain.MessageProvider{"sms": mockProvider}

	hub := NewHub()
	go hub.Run()

	handler := NewConversationHandler(convRepo, msgRepo, channelRepo, nil, providers, hub, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	conversations := router.Group("/v1/conversations")
	conversations.Use(authMw, middleware.RequireRole("admin", "attendant"))
	{
		conversations.POST("/:id/reply", handler.HandleHumanReply)
	}

	body := map[string]string{"message": "did this arrive?"}
	w := performRequest(router, http.MethodPost, "/v1/conversations/conv-1/reply", body, testToken)

	assert.Equal(t, http.StatusBadGateway, w.Code, "send failure must not be a silent 200")

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "MESSAGE_SEND_FAILED", resp["code"])
	assert.NotEqual(t, "reply_sent", resp["status"])

	msgRepo.AssertCalled(t, "MarkSendFailed", mock.Anything, persistedID, "twilio: 21610 blocked recipient")
	convRepo.AssertExpectations(t)
	msgRepo.AssertExpectations(t)
	mockProvider.AssertExpectations(t)
}

// US-010 (review fixes): the 24h customer-service window is a Meta
// restriction on WhatsApp AND Instagram. The old gate checked
// channel.Type == "whatsapp" literally, so a >24h Instagram reply went
// through (Meta rejects it upstream) while SMS/Telegram were fine only
// by accident of the same literal.
func TestHumanReply_24hWindowByChannelCapability(t *testing.T) {
	stale := time.Now().Add(-25 * time.Hour)

	cases := []struct {
		channelType string
		wantBlocked bool
	}{
		{"instagram", true}, // FAILED before US-010: whatsapp-only literal let it through
		{"whatsapp", true},
		{"sms", false},
		{"telegram", false},
	}
	for _, tc := range cases {
		t.Run(tc.channelType, func(t *testing.T) {
			convRepo := new(MockConversationRepo)
			msgRepo := new(MockMessageRepo)
			channelRepo := new(MockChannelRepo)
			mockProvider := new(MockMessageProvider)
			jwtMock := new(MockJWTService)
			expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

			convRepo.On("GetByID", mock.Anything, "conv-1", testAccountID).Return(&domain.Conversation{
				ID:                    "conv-1",
				AccountID:             testAccountID,
				ChannelID:             "ch-1",
				ContactID:             "contact-1",
				Status:                domain.ConversationOpen,
				LastCustomerMessageAt: &stale,
			}, nil)
			channelRepo.On("GetByID", mock.Anything, "ch-1").Return(&domain.Channel{
				AccountID: testAccountID,
				ID:        "ch-1",
				Type:      tc.channelType,
			}, nil)

			if !tc.wantBlocked {
				convRepo.On("SetBotPaused", mock.Anything, "conv-1", testAccountID, true).Return(nil)
				convRepo.On("ClearWaitingSince", mock.Anything, "conv-1", testAccountID).Return(nil)
				msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Message")).Return(nil)
				mockProvider.On("SendMessage", mock.Anything, mock.Anything, "contact-1", mock.AnythingOfType("*domain.Message")).Return(nil)
			}

			providers := map[string]domain.MessageProvider{tc.channelType: mockProvider}
			handler := NewConversationHandler(convRepo, msgRepo, channelRepo, nil, providers, nil, nil)

			router := setupTestRouter()
			authMw := setupAuthMiddleware(jwtMock)
			conversations := router.Group("/v1/conversations")
			conversations.Use(authMw, middleware.RequireRole("admin", "attendant"))
			conversations.POST("/:id/reply", handler.HandleHumanReply)

			body := map[string]string{"message": "reply >24h after last customer message"}
			w := performRequest(router, http.MethodPost, "/v1/conversations/conv-1/reply", body, testToken)

			if tc.wantBlocked {
				assert.Equal(t, http.StatusForbidden, w.Code,
					"%s has a Meta 24h window — stale reply must be 403", tc.channelType)
				var resp map[string]interface{}
				_ = json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Equal(t, "CONV_OUTSIDE_24H", resp["code"])
				msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
				mockProvider.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			} else {
				assert.Equal(t, http.StatusOK, w.Code,
					"%s has no 24h window — stale reply must go through", tc.channelType)
				mockProvider.AssertExpectations(t)
			}
		})
	}
}

// US-003 canal-interno: a human reply on an internal CS thread must reach
// BOTH sides — the platform inbox (standard new_message) and the customer
// account's support widget (support_message tagged with the thread owner).
func TestHumanReply_InternalChannel_MirrorsToCustomerWidget(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	custAcc := "01CUSTOMERACCOUNT000000000"
	threadOwner := "01CUSTOMERUSER000000000000"
	now := time.Now()
	convRepo.On("GetByID", mock.Anything, "conv-int", testAccountID).Return(&domain.Conversation{
		ID:                "conv-int",
		AccountID:         testAccountID,
		ChannelID:         "ch-int",
		ContactID:         threadOwner,
		Status:            domain.ConversationOpen,
		CustomerAccountID: &custAcc,
		CreatedAt:         now,
	}, nil)
	convRepo.On("SetBotPaused", mock.Anything, "conv-int", testAccountID, true).Return(nil)
	convRepo.On("ClearWaitingSince", mock.Anything, "conv-int", testAccountID).Return(nil)
	msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Message")).Return(nil)
	channelRepo.On("GetByID", mock.Anything, "ch-int").Return(&domain.Channel{
		ID:        "ch-int",
		AccountID: testAccountID,
		Type:      domain.ChannelTypeInternal,
		Name:      "Suporte Replider",
	}, nil)

	hub := NewHub()
	go hub.Run()
	platformClient := &Client{Hub: hub, Send: make(chan []byte, 8), AccountID: testAccountID, UserID: testUserID}
	customerClient := &Client{Hub: hub, Send: make(chan []byte, 8), AccountID: custAcc, UserID: threadOwner}
	hub.Register <- platformClient
	hub.Register <- customerClient
	time.Sleep(10 * time.Millisecond)

	// Real inapp adapter wired exactly like main.go: hub as broadcaster,
	// the conversation repo as resolver.
	inappAdapter := inapp.NewAdapter()
	inappAdapter.SetBroadcaster(hub, convRepo)
	providers := map[string]domain.MessageProvider{"internal": inappAdapter}

	handler := NewConversationHandler(convRepo, msgRepo, channelRepo, nil, providers, hub, nil)
	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	conversations := router.Group("/v1/conversations")
	conversations.Use(authMw, middleware.RequireRole("admin", "attendant"))
	{
		conversations.POST("/:id/reply", handler.HandleHumanReply)
	}

	body := map[string]string{"message": "resposta do vendedor"}
	w := performRequest(router, http.MethodPost, "/v1/conversations/conv-int/reply", body, testToken)
	assert.Equal(t, http.StatusOK, w.Code)

	select {
	case payload := <-platformClient.Send:
		var evt map[string]interface{}
		assert.NoError(t, json.Unmarshal(payload, &evt))
		assert.Equal(t, "new_message", evt["event"])
		assert.Equal(t, "conv-int", evt["conversation_id"])
	case <-time.After(time.Second):
		t.Fatal("platform client never received the new_message broadcast")
	}

	select {
	case payload := <-customerClient.Send:
		var evt map[string]interface{}
		assert.NoError(t, json.Unmarshal(payload, &evt))
		assert.Equal(t, domain.SupportMessageEvent, evt["event"])
		assert.Equal(t, true, evt["internal_thread"])
		assert.Equal(t, threadOwner, evt["user_id"])
		assert.Equal(t, "conv-int", evt["conversation_id"])
	case <-time.After(time.Second):
		t.Fatal("customer client never received the support_message mirror")
	}
}

// The channel filter is the SDR inbox's way of separating email from
// WhatsApp in ONE list. It must reach the repository, because filtering an
// already-paginated page in Go would drop every email thread ranking behind
// a page of WhatsApp — the inbox would report "no email" for an account
// full of it.
func TestListConversations_ChannelFilterReachesTheRepository(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	convRepo.On("ListEnriched", mock.Anything, testAccountID, "open", int32(50), int32(0), "", "", "email").
		Return([]*domain.ConversationSummary{}, nil)

	router := setupConversationTestRouter(convRepo, msgRepo, channelRepo, jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/conversations?channel=email", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)
	convRepo.AssertExpectations(t)
}

// An unknown channel is a 400, not a silently empty inbox: an empty list
// reads as "you have no email", which is a different and much worse answer
// than "that filter does not exist".
func TestListConversations_InvalidChannelReturns400(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	router := setupConversationTestRouter(convRepo, msgRepo, channelRepo, jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/conversations?channel=carrier-pigeon", nil, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp domain.AppError
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_FILTER", resp.Code)

	convRepo.AssertNotCalled(t, "ListEnriched", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A human reply on an EMAIL channel must thread onto the customer's last
// inbound (In-Reply-To / References / "Re: " subject) and go to its
// Reply-To when one was given — otherwise Outlook files the answer as a
// loose message outside the thread.
func TestHumanReply_EmailThreadsOntoLastInbound(t *testing.T) {
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	mockProvider := new(MockMessageProvider)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	now := time.Now()
	convRepo.On("GetByID", mock.Anything, "conv-1", testAccountID).Return(&domain.Conversation{
		ID: "conv-1", AccountID: testAccountID, ChannelID: "ch-1",
		ContactID: "comprador@hospital.br", Status: domain.ConversationOpen, CreatedAt: now,
	}, nil)
	convRepo.On("SetBotPaused", mock.Anything, "conv-1", testAccountID, true).Return(nil)
	convRepo.On("ClearWaitingSince", mock.Anything, "conv-1", testAccountID).Return(nil)
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(&domain.Channel{
		AccountID: testAccountID, ID: "ch-1", Type: "email", Name: "Mailbox",
	}, nil)
	msgRepo.On("LastInboundInConversation", mock.Anything, "conv-1").Return(&domain.Message{
		Direction: "inbound",
		Metadata:  json.RawMessage(`{"message_id":"<in@hospital.br>","references":"<root@replider>","subject":"Proposta","reply_to":"compras@hospital.br"}`),
	}, nil)
	msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
		meta, _ := m.Metadata.(map[string]any)
		return meta["in_reply_to"] == "<in@hospital.br>" &&
			meta["references"] == "<root@replider> <in@hospital.br>" &&
			meta["subject"] == "Re: Proposta"
	})).Return(true, nil)
	mockProvider.On("SendMessage", mock.Anything, mock.Anything, "compras@hospital.br", mock.MatchedBy(func(m *domain.Message) bool {
		meta, _ := m.Metadata.(map[string]any)
		return meta["in_reply_to"] == "<in@hospital.br>"
	})).Return(nil)

	handler := NewConversationHandler(convRepo, msgRepo, channelRepo, nil,
		map[string]domain.MessageProvider{"email": mockProvider}, nil, nil)
	router := setupTestRouter()
	conversations := router.Group("/v1/conversations")
	conversations.Use(setupAuthMiddleware(jwtMock), middleware.RequireRole("admin", "attendant"))
	conversations.POST("/:id/reply", handler.HandleHumanReply)

	w := performRequest(router, http.MethodPost, "/v1/conversations/conv-1/reply",
		map[string]string{"message": "Segue a proposta."}, testToken)

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	msgRepo.AssertExpectations(t)
	mockProvider.AssertExpectations(t)
}
