package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// inboxPortsRepo is MockConversationRepo plus the three optional ports the
// postgres repository satisfies (ConversationLister, ConversationReopener,
// UnreadCounter). The plain mock deliberately lacks them, which is what
// keeps every other handler test on the legacy paths.
type inboxPortsRepo struct {
	*MockConversationRepo
	listFilter   *domain.ConversationListFilter
	listRows     []*domain.ConversationSummary
	latest       *domain.Conversation
	reopenResult bool
	reopenErr    error
	reopened     []string
	unreadConvs  int64
	unreadMsgs   int64
}

func (r *inboxPortsRepo) ListEnrichedQuery(_ context.Context, f domain.ConversationListFilter) ([]*domain.ConversationSummary, error) {
	r.listFilter = &f
	return r.listRows, nil
}

func (r *inboxPortsRepo) FindLatestByContact(_ context.Context, _, _ string) (*domain.Conversation, error) {
	return r.latest, nil
}

func (r *inboxPortsRepo) Reopen(_ context.Context, conversationID, _ string) (bool, error) {
	r.reopened = append(r.reopened, conversationID)
	return r.reopenResult, r.reopenErr
}

func (r *inboxPortsRepo) CountUnread(_ context.Context, _ string) (int64, int64, error) {
	return r.unreadConvs, r.unreadMsgs, nil
}

func setupInboxPortsRouter(repo *inboxPortsRepo, jwtMock *MockJWTService) *gin.Engine {
	handler := NewConversationHandler(repo, new(MockMessageRepo), new(MockChannelRepo), nil, nil, nil, nil)
	router := setupTestRouter()
	g := router.Group("/v1/conversations")
	g.Use(setupAuthMiddleware(jwtMock), middleware.RequireRole("admin", "attendant"))
	{
		g.GET("", handler.HandleListConversations)
		g.GET("/unread-count", handler.HandleUnreadCount)
		g.PATCH("/:id/reopen", handler.HandleReopenConversation)
	}
	return router
}

func doInboxRequest(router *gin.Engine, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestReopenConversation_ClosedBecomesOpen(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	repo := &inboxPortsRepo{MockConversationRepo: new(MockConversationRepo), reopenResult: true}
	resolved := time.Now()
	repo.On("GetByID", mock.Anything, "conv-1", testAccountID).Return(&domain.Conversation{
		ID: "conv-1", AccountID: testAccountID, Status: domain.ConversationClosed, ResolvedAt: &resolved,
	}, nil)

	w := doInboxRequest(setupInboxPortsRouter(repo, jwtMock), http.MethodPatch, "/v1/conversations/conv-1/reopen")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, []string{"conv-1"}, repo.reopened)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "open", body["status"])
}

func TestReopenConversation_AlreadyOpenIsANoop(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	repo := &inboxPortsRepo{MockConversationRepo: new(MockConversationRepo)}
	repo.On("GetByID", mock.Anything, "conv-1", testAccountID).Return(&domain.Conversation{
		ID: "conv-1", AccountID: testAccountID, Status: domain.ConversationOpen,
	}, nil)

	w := doInboxRequest(setupInboxPortsRouter(repo, jwtMock), http.MethodPatch, "/v1/conversations/conv-1/reopen")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Empty(t, repo.reopened, "an open thread must not hit the UPDATE")
}

func TestReopenConversation_AnotherOpenThreadIs409(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	repo := &inboxPortsRepo{MockConversationRepo: new(MockConversationRepo), reopenErr: domain.ErrConversationAlreadyOpen}
	repo.On("GetByID", mock.Anything, "conv-1", testAccountID).Return(&domain.Conversation{
		ID: "conv-1", AccountID: testAccountID, Status: domain.ConversationClosed,
	}, nil)

	w := doInboxRequest(setupInboxPortsRouter(repo, jwtMock), http.MethodPatch, "/v1/conversations/conv-1/reopen")

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}

func TestReopenConversation_UnknownThreadIs404(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	repo := &inboxPortsRepo{MockConversationRepo: new(MockConversationRepo)}
	repo.On("GetByID", mock.Anything, "ghost", testAccountID).Return(nil, nil)

	w := doInboxRequest(setupInboxPortsRouter(repo, jwtMock), http.MethodPatch, "/v1/conversations/ghost/reopen")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Empty(t, repo.reopened)
}

func TestUnreadCount_ReportsConversationsAndMessages(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "attendant")
	repo := &inboxPortsRepo{MockConversationRepo: new(MockConversationRepo), unreadConvs: 3, unreadMsgs: 7}

	w := doInboxRequest(setupInboxPortsRouter(repo, jwtMock), http.MethodGet, "/v1/conversations/unread-count")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body map[string]int64
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, int64(3), body["count"])
	assert.Equal(t, int64(7), body["messages"])
}

func TestUnreadCount_WithoutThePortIs503(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	router := setupConversationTestRouter(new(MockConversationRepo), new(MockMessageRepo), new(MockChannelRepo), jwtMock)
	handler := NewConversationHandler(new(MockConversationRepo), new(MockMessageRepo), new(MockChannelRepo), nil, nil, nil, nil)
	router.GET("/v1/conversations/unread-count", setupAuthMiddleware(jwtMock), handler.HandleUnreadCount)

	w := doInboxRequest(router, http.MethodGet, "/v1/conversations/unread-count")

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// Search and priority reach the SQL as part of the filter, and the page the
// repository returns is served AS IS — the Go filter over the page is what
// answered "nada encontrado" for a name on page two.
func TestListConversations_SearchAndPriorityRunInSQL(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	repo := &inboxPortsRepo{
		MockConversationRepo: new(MockConversationRepo),
		listRows: []*domain.ConversationSummary{
			// Would fail a Go-side name match on "alice": the SQL matched it
			// on the last message, which the handler cannot see.
			{ID: "conv-1", AccountID: testAccountID, ContactName: "Bob", Priority: "high", UnreadCount: 2},
		},
	}

	w := doInboxRequest(setupInboxPortsRouter(repo, jwtMock), http.MethodGet,
		"/v1/conversations?q=alice&priority=high&limit=20&offset=40&sort=oldest&channel=email")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotNil(t, repo.listFilter)
	assert.Equal(t, "alice", repo.listFilter.Search)
	assert.Equal(t, "high", repo.listFilter.Priority)
	assert.Equal(t, int32(20), repo.listFilter.Limit)
	assert.Equal(t, int32(40), repo.listFilter.Offset)
	assert.Equal(t, "oldest", repo.listFilter.Sort)
	assert.Equal(t, "email", repo.listFilter.ChannelType)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rows))
	require.Len(t, rows, 1, "the SQL-filtered page must not be filtered again in Go")
	assert.EqualValues(t, 2, rows[0]["unread_count"])
}

// The legacy path (a repository without ConversationLister) keeps filtering
// the page in Go, so older mocks and adapters behave exactly as before.
func TestListConversations_LegacyRepoStillFiltersThePage(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	convRepo := new(MockConversationRepo)
	convRepo.On("ListEnriched", mock.Anything, testAccountID, "open", int32(50), int32(0), "", "", "").
		Return([]*domain.ConversationSummary{
			{ID: "a", ContactName: "Alice", Priority: "normal"},
			{ID: "b", ContactName: "Bob", Priority: "normal"},
		}, nil)
	router := setupConversationTestRouter(convRepo, new(MockMessageRepo), new(MockChannelRepo), jwtMock)

	w := doInboxRequest(router, http.MethodGet, "/v1/conversations?q=alice")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "a", rows[0]["id"])
}

func TestInboundPushPayload_ClickURLOpensTheInboxThread(t *testing.T) {
	assert.Equal(t, "/home?conversation=conv-9", composeHandoffPushPayload("acc", "conv-9", "Ana").ClickURL)
}
