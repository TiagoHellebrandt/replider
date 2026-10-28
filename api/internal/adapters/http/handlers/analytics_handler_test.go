package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetAnalytics_Success_Handler(t *testing.T) {
	analyticsRepo := new(MockAnalyticsRepo)
	usageRepo := new(MockUsageRepo)
	planRepo := new(MockPlanRepo)
	accountRepo := new(MockAccountRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	analyticsRepo.On("GetConversationStats", mock.Anything, testAccountID).Return(&domain.ConversationStats{
		Total: 50, Open: 10, PendingHuman: 3, Closed: 37, Last24h: 5, Last7d: 20,
	}, nil)
	analyticsRepo.On("GetMessageStats", mock.Anything, testAccountID).Return(&domain.MessageStats{
		Total: 200, Bot: 80, Human: 40, Contact: 80, Last24h: 25,
	}, nil)
	analyticsRepo.On("GetDailyConversationCounts", mock.Anything, testAccountID).Return([]domain.DailyCount{
		{Date: time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC), Count: 5},
	}, nil)
	analyticsRepo.On("GetSLAMetrics", mock.Anything, testAccountID).Return(&domain.SLAMetrics{
		AvgFirstResponseSeconds: 60, PctRespondedWithin5Min: 90, TotalResolved: 30, TotalResponded: 40,
	}, nil)
	analyticsRepo.On("GetAgentPerformance", mock.Anything, testAccountID).Return([]domain.AgentPerformance{}, nil)
	analyticsRepo.On("GetAIPerformance", mock.Anything, testAccountID).Return(&domain.AIPerformance{}, nil)

	tokensUsed := int64(1500)
	addonTokens := int64(0)
	usageRepo.On("GetByAccountID", mock.Anything, testAccountID).Return(&domain.AccountUsage{
		ID:           "usage-1",
		AccountID:    testAccountID,
		AITokensUsed: &tokensUsed,
		AddonTokens:  &addonTokens,
	}, nil)

	planID := "plan-1"
	accountRepo.On("GetByID", mock.Anything, testAccountID).Return(&domain.Account{
		ID:     testAccountID,
		PlanID: &planID,
	}, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(&domain.Plan{
		ID:                  planID,
		Name:                "Starter",
		MaxAITokensPerMonth: 5000,
	}, nil)

	getAnalyticsUC := usecases.NewGetAnalyticsUseCase(analyticsRepo, usageRepo, planRepo, accountRepo)
	handler := NewAnalyticsHandler(getAnalyticsUC, nil, nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.GET("/v1/analytics", authMw, middleware.RequireRole("admin"), handler.HandleGetAnalytics)

	w := performRequest(router, http.MethodGet, "/v1/analytics", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp usecases.AnalyticsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, int64(50), resp.Conversations.Total)
	assert.Equal(t, int64(10), resp.Conversations.Open)
	assert.Equal(t, int64(200), resp.Messages.Total)
	assert.Len(t, resp.DailyActivity, 1)
	assert.Equal(t, int64(1500), resp.TokensUsed)
	assert.Equal(t, int64(5000), resp.TokensLimit)

	analyticsRepo.AssertExpectations(t)
	usageRepo.AssertExpectations(t)
	planRepo.AssertExpectations(t)
	accountRepo.AssertExpectations(t)
}

func TestGetAnalytics_Error_Handler(t *testing.T) {
	analyticsRepo := new(MockAnalyticsRepo)
	usageRepo := new(MockUsageRepo)
	planRepo := new(MockPlanRepo)
	accountRepo := new(MockAccountRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	analyticsRepo.On("GetConversationStats", mock.Anything, testAccountID).Return((*domain.ConversationStats)(nil), errors.New("db error"))

	getAnalyticsUC := usecases.NewGetAnalyticsUseCase(analyticsRepo, usageRepo, planRepo, accountRepo)
	handler := NewAnalyticsHandler(getAnalyticsUC, nil, nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.GET("/v1/analytics", authMw, middleware.RequireRole("admin"), handler.HandleGetAnalytics)

	w := performRequest(router, http.MethodGet, "/v1/analytics", nil, testToken)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INTERNAL_ERROR", resp["code"])

	analyticsRepo.AssertExpectations(t)
}

func TestListAttendantPerformance_AdminSeesAll(t *testing.T) {
	analyticsRepo := new(MockAnalyticsRepo)
	userRepo := new(MockUserRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	adminRole := "admin"
	userRepo.On("GetByID", mock.Anything, testUserID).Return(&domain.User{
		ID: testUserID, AccountID: testAccountID, Role: &adminRole,
	}, nil)

	analyticsRepo.On("GetAttendantPerformance", mock.Anything, mock.MatchedBy(func(f domain.AttendantPerformanceFilter) bool {
		return f.AccountID == testAccountID && f.UserID == nil
	})).Return([]domain.AttendantPerformance{
		{UserID: "u1", Name: "Ana", ConversationsAssigned: 12, CSATAvg: 4.5, CSATCount: 8},
		{UserID: "u2", Name: "Bruno", ConversationsAssigned: 3},
	}, nil)

	uc := usecases.NewGetAttendantPerformanceUseCase(analyticsRepo, userRepo)
	handler := NewAnalyticsHandler(nil, uc, nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	g := router.Group("/v1/analytics", authMw, middleware.RequireRole("admin", "attendant"))
	g.GET("/attendants", handler.HandleListAttendantPerformance)

	w := performRequest(router, http.MethodGet, "/v1/analytics/attendants?period=7d", nil, testToken)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp []domain.AttendantPerformance
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "Ana", resp[0].Name)
}

func TestListAttendantPerformance_AttendantPeerBlocked(t *testing.T) {
	analyticsRepo := new(MockAnalyticsRepo)
	userRepo := new(MockUserRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "attendant")

	att := "attendant"
	userRepo.On("GetByID", mock.Anything, testUserID).Return(&domain.User{
		ID: testUserID, AccountID: testAccountID, Role: &att,
	}, nil)

	uc := usecases.NewGetAttendantPerformanceUseCase(analyticsRepo, userRepo)
	handler := NewAnalyticsHandler(nil, uc, nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	g := router.Group("/v1/analytics", authMw, middleware.RequireRole("admin", "attendant"))
	g.GET("/attendants", handler.HandleListAttendantPerformance)

	// user_id=somebody-else → 403: a credencial está boa, o alcance é que não.
	// Com 401 o app entende "sua sessão morreu" e apaga a sessão inteira.
	w := performRequest(router, http.MethodGet, "/v1/analytics/attendants?user_id=someone-else", nil, testToken)
	assert.Equal(t, http.StatusForbidden, w.Code)
	analyticsRepo.AssertNotCalled(t, "GetAttendantPerformance")
}

func TestExportAttendantPerformanceCSV(t *testing.T) {
	analyticsRepo := new(MockAnalyticsRepo)
	userRepo := new(MockUserRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	adminRole := "admin"
	userRepo.On("GetByID", mock.Anything, testUserID).Return(&domain.User{
		ID: testUserID, AccountID: testAccountID, Role: &adminRole,
	}, nil)

	lastSeen := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	analyticsRepo.On("GetAttendantPerformance", mock.Anything, mock.Anything).Return([]domain.AttendantPerformance{
		{
			UserID: "u1", Name: "Ana", Email: "ana@acme.com", Role: "attendant",
			LastSeenAt:              &lastSeen,
			ConversationsAssigned:   12,
			ConversationsResolved:   9,
			MessagesSent:            58,
			AvgFirstResponseSeconds: 120,
			AvgResolutionSeconds:    3600,
			CSATAvg:                 4.5,
			CSATCount:               7,
		},
	}, nil)

	uc := usecases.NewGetAttendantPerformanceUseCase(analyticsRepo, userRepo)
	handler := NewAnalyticsHandler(nil, uc, nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	g := router.Group("/v1/analytics", authMw, middleware.RequireRole("admin", "attendant"))
	g.GET("/attendants/export.csv", handler.HandleExportAttendantPerformanceCSV)

	w := performRequest(router, http.MethodGet, "/v1/analytics/attendants/export.csv", nil, testToken)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	body := w.Body.String()
	assert.Contains(t, body, "user_id,name,email,role,last_seen_at,conversations_assigned")
	assert.Contains(t, body, "Ana,ana@acme.com,attendant")
	assert.Contains(t, body, "4.50")
}
