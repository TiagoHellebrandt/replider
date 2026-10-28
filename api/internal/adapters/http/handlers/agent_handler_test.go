package handlers

import (
	"encoding/json"
	"errors"
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

const (
	testToken     = "valid-jwt-token"
	testUserID    = "user-1"
	testAccountID = "acc-1"
	testAgentID   = "agent-1"
)

func setupAgentTestRouter(agentRepo *MockAgentRepo, jwtMock *MockJWTService) *AgentHandler {
	createUC := usecases.NewCreateAgentUseCase(agentRepo, nil, nil)
	listUC := usecases.NewListAgentsUseCase(agentRepo)
	updateUC := usecases.NewUpdateAgentUseCase(agentRepo)
	toggleUC := usecases.NewToggleAgentStatusUseCase(agentRepo)
	deleteUC := usecases.NewDeleteAgentUseCase(agentRepo)

	return NewAgentHandler(nil, createUC, listUC, updateUC, toggleUC, deleteUC, nil)
}

func registerAgentRoutes(router *gin.Engine, handler *AgentHandler, jwtMock *MockJWTService) {
	authMw := setupAuthMiddleware(jwtMock)
	agents := router.Group("/v1/agents")
	agents.Use(authMw, middleware.RequireRole("admin"))
	{
		agents.POST("", handler.HandleCreateAgent)
		agents.GET("", handler.HandleListAgents)
		agents.PUT("/:id", handler.HandleUpdateAgent)
		agents.PATCH("/:id/toggle", handler.HandlePauseAgent)
		agents.DELETE("/:id", handler.HandleDeleteAgent)
	}
}

func TestCreateAgent_Success(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	// No existing agents (limit not reached)
	agentRepo.On("ListByAccountID", mock.Anything, testAccountID).Return([]*domain.Agent{}, nil)
	agentRepo.On("Create", mock.Anything, mock.MatchedBy(func(a *domain.Agent) bool {
		return a.Name == "My Bot" && a.AccountID == testAccountID && len(a.ID) == 26
	})).Return(nil)

	handler := setupAgentTestRouter(agentRepo, jwtMock)
	router := setupTestRouter()
	registerAgentRoutes(router, handler, jwtMock)

	body := map[string]interface{}{
		"name":     "My Bot",
		"provider": "openai",
		"model":    "gpt-4o",
		"timezone": "America/Sao_Paulo",
	}
	w := performRequest(router, http.MethodPost, "/v1/agents", body, testToken)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp domain.Agent
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "My Bot", resp.Name)
	assert.Equal(t, testAccountID, resp.AccountID)
	assert.True(t, resp.IsActive)

	agentRepo.AssertExpectations(t)
	jwtMock.AssertExpectations(t)
}

func TestCreateAgent_LimitReached(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	// Already has one agent
	agentRepo.On("ListByAccountID", mock.Anything, testAccountID).Return([]*domain.Agent{
		{ID: "existing-agent", AccountID: testAccountID, Name: "Existing"},
	}, nil)

	handler := setupAgentTestRouter(agentRepo, jwtMock)
	router := setupTestRouter()
	registerAgentRoutes(router, handler, jwtMock)

	body := map[string]interface{}{"name": "Another Bot"}
	w := performRequest(router, http.MethodPost, "/v1/agents", body, testToken)

	assert.Equal(t, http.StatusConflict, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "PLAN_LIMIT_EXCEEDED", resp["code"])
	// Metadata carries the resource + counts so the Flutter client can
	// route into the upgrade modal with precise copy.
	meta, ok := resp["metadata"].(map[string]interface{})
	assert.True(t, ok, "expected metadata object")
	assert.Equal(t, "agents", meta["resource"])
}

func TestCreateAgent_Unauthorized(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)

	handler := setupAgentTestRouter(agentRepo, jwtMock)
	router := setupTestRouter()
	registerAgentRoutes(router, handler, jwtMock)

	body := map[string]interface{}{"name": "My Bot"}
	w := performRequest(router, http.MethodPost, "/v1/agents", body, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListAgents_Success(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agents := []*domain.Agent{
		{ID: "agent-1", AccountID: testAccountID, Name: "Bot 1", IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "agent-2", AccountID: testAccountID, Name: "Bot 2", IsActive: false, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	agentRepo.On("ListByAccountID", mock.Anything, testAccountID).Return(agents, nil)

	handler := setupAgentTestRouter(agentRepo, jwtMock)
	router := setupTestRouter()
	registerAgentRoutes(router, handler, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/agents", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []domain.Agent
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "Bot 1", resp[0].Name)
}

func TestUpdateAgent_NotOwned(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	// Agent belongs to a different account
	agentRepo.On("GetByID", mock.Anything, "agent-other").Return(&domain.Agent{
		ID:        "agent-other",
		AccountID: "other-account",
		Name:      "Not Mine",
	}, nil)

	handler := setupAgentTestRouter(agentRepo, jwtMock)
	router := setupTestRouter()
	registerAgentRoutes(router, handler, jwtMock)

	body := map[string]interface{}{"name": "Hacked Name"}
	w := performRequest(router, http.MethodPut, "/v1/agents/agent-other", body, testToken)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "NOT_FOUND", resp["code"])
}

func TestDeleteAgent_Success(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo.On("GetByID", mock.Anything, testAgentID).Return(&domain.Agent{
		ID:        testAgentID,
		AccountID: testAccountID,
		Name:      "My Bot",
	}, nil)
	agentRepo.On("Delete", mock.Anything, testAgentID).Return(nil)

	handler := setupAgentTestRouter(agentRepo, jwtMock)
	router := setupTestRouter()
	registerAgentRoutes(router, handler, jwtMock)

	w := performRequest(router, http.MethodDelete, "/v1/agents/"+testAgentID, nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "agent deleted successfully", resp["message"])

	agentRepo.AssertExpectations(t)
}

func TestAgent_RBAC_AttendantDenied(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "attendant") // attendant role

	handler := setupAgentTestRouter(agentRepo, jwtMock)
	router := setupTestRouter()
	registerAgentRoutes(router, handler, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/agents", nil, testToken)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "FORBIDDEN", resp["code"])
}

func TestListAgents_InternalError(t *testing.T) {
	agentRepo := new(MockAgentRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo.On("ListByAccountID", mock.Anything, testAccountID).Return(([]*domain.Agent)(nil), errors.New("db connection failed"))

	handler := setupAgentTestRouter(agentRepo, jwtMock)
	router := setupTestRouter()
	registerAgentRoutes(router, handler, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/agents", nil, testToken)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
