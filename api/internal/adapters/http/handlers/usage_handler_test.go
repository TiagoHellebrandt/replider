package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetUsage_Success(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	usageRepo := new(MockUsageRepo)
	planRepo := new(MockPlanRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	planID := "plan-1"
	accountRepo.On("GetByID", mock.Anything, testAccountID).Return(&domain.Account{
		ID:     testAccountID,
		Name:   "Test Co",
		PlanID: &planID,
	}, nil)

	tokensUsed := int64(1500)
	inputTokens := int64(1000)
	outputTokens := int64(500)
	convCount := int32(10)
	addonTokens := int64(0)
	usageRepo.On("GetSummaryByAccountID", mock.Anything, testAccountID).Return(&domain.AccountUsage{
		ID:                 "usage-1",
		AccountID:          testAccountID,
		AITokensUsed:       &tokensUsed,
		AIInputTokensUsed:  &inputTokens,
		AIOutputTokensUsed: &outputTokens,
		ConversationsCount: &convCount,
		AddonTokens:        &addonTokens,
		UpdatedAt:          time.Now(),
	}, nil)

	planRepo.On("GetByID", mock.Anything, planID).Return(&domain.Plan{
		ID:                       planID,
		Name:                     "Starter",
		MaxAITokensPerMonth:      5000,
		MaxConversationsPerMonth: 50,
	}, nil)

	getUsageUC := usecases.NewGetUsageUseCase(usageRepo, planRepo, accountRepo)
	handler := NewUsageHandler(getUsageUC)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.GET("/v1/accounts/usage", authMw, middleware.RequireRole("admin"), handler.HandleGetUsage)

	w := performRequest(router, http.MethodGet, "/v1/accounts/usage", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp usecases.GetUsageResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, int64(1500), resp.AITokensUsed)
	assert.Equal(t, int64(1000), resp.AIInputTokensUsed)
	assert.Equal(t, int64(500), resp.AIOutputTokensUsed)
	assert.Equal(t, int32(10), resp.ConversationsCount)
	assert.Equal(t, int64(5000), resp.MaxAITokensPerMonth)
	assert.Equal(t, 50, resp.MaxConversationsPerMonth)

	accountRepo.AssertExpectations(t)
	usageRepo.AssertExpectations(t)
	planRepo.AssertExpectations(t)
}

func TestGetUsage_AccountNotFound(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	usageRepo := new(MockUsageRepo)
	planRepo := new(MockPlanRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	accountRepo.On("GetByID", mock.Anything, testAccountID).Return(nil, nil)

	getUsageUC := usecases.NewGetUsageUseCase(usageRepo, planRepo, accountRepo)
	handler := NewUsageHandler(getUsageUC)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.GET("/v1/accounts/usage", authMw, middleware.RequireRole("admin"), handler.HandleGetUsage)

	w := performRequest(router, http.MethodGet, "/v1/accounts/usage", nil, testToken)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "ACCOUNT_NOT_FOUND", resp["code"])

	accountRepo.AssertExpectations(t)
}

func TestGetUsage_AttendantDenied(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "attendant")

	handler := NewUsageHandler(nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.GET("/v1/accounts/usage", authMw, middleware.RequireRole("admin"), handler.HandleGetUsage)

	w := performRequest(router, http.MethodGet, "/v1/accounts/usage", nil, testToken)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "FORBIDDEN", resp["code"])
}
