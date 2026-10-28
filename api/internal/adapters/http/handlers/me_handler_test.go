package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

func setupMeTestRouter(
	userRepo *MockUserRepo,
	accountRepo *MockAccountRepo,
	jwtMock *MockJWTService,
) (*MeHandler, *gin.Engine) {
	planRepo := new(MockPlanRepo)
	getMeUC := usecases.NewGetMeUseCase(userRepo, accountRepo, planRepo, nil)
	updateProfileUC := usecases.NewUpdateProfileUseCase(userRepo)
	changePasswordUC := usecases.NewChangePasswordUseCase(userRepo)
	handler := NewMeHandler(getMeUC, updateProfileUC, changePasswordUC, nil, nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	me := router.Group("/v1/me")
	me.Use(authMw)
	{
		me.GET("", handler.HandleGetMe)
		me.PUT("", handler.HandleUpdateProfile)
		me.PATCH("/password", handler.HandleChangePassword)
	}
	return handler, router
}

func TestGetMe_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	role := "admin"
	userRepo.On("GetByID", mock.Anything, testUserID).Return(&domain.User{
		ID:        testUserID,
		AccountID: testAccountID,
		Name:      "John",
		Email:     "john@example.com",
		Role:      &role,
		CreatedAt: time.Now(),
	}, nil)
	accountRepo.On("GetByID", mock.Anything, testAccountID).Return(&domain.Account{
		ID:                 testAccountID,
		Name:               "Test Co",
		SubscriptionStatus: "trialing",
		CreatedAt:          time.Now(),
	}, nil)

	_, router := setupMeTestRouter(userRepo, accountRepo, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/me", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp usecases.GetMeResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, testUserID, resp.User.ID)
	assert.Equal(t, "John", resp.User.Name)
	assert.Equal(t, testAccountID, resp.Account.ID)

	userRepo.AssertExpectations(t)
	accountRepo.AssertExpectations(t)
}

func TestGetPlanLimits_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	planID := "plan-pro"
	stripeID := "price_pro"
	accountRepo.On("GetByID", mock.Anything, testAccountID).Return(&domain.Account{
		ID:     testAccountID,
		PlanID: &planID,
	}, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(&domain.Plan{
		ID:                           planID,
		Name:                         "Pro",
		StripePriceID:                &stripeID,
		MaxAgents:                    5,
		MaxChannels:                  8,
		MaxSeats:                     5,
		MaxConversationsPerMonth:     2000,
		MaxAITokensPerMonth:          800000,
		MaxStorageMB:                 102400,
		MaxBroadcastMessagesPerMonth: 0,
		OverageConversationCents:     50,
		OverageCapMultiplier:         2.0,
	}, nil)

	getMeUC := usecases.NewGetMeUseCase(userRepo, accountRepo, planRepo, nil)
	handler := NewMeHandler(getMeUC, nil, nil, nil, nil, nil, nil)
	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.GET("/v1/me/plan-limits", authMw, handler.HandleGetPlanLimits)

	w := performRequest(router, http.MethodGet, "/v1/me/plan-limits", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp usecases.EffectivePlanLimits
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Pro", resp.PlanName)
	assert.False(t, resp.IsContractPlan)
	assert.Equal(t, 5, resp.MaxAgents)
	assert.Equal(t, 2000, resp.MaxConversationsPerMonth)
	assert.Equal(t, int64(800000), resp.MaxAITokensPerMonth)
	assert.Equal(t, 0, resp.MaxBroadcastMessagesPerMonth)
	assert.Equal(t, 50, resp.OverageConversationCents)
}

func TestUpdateProfile_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	// Email check: no other user has this email
	userRepo.On("GetByEmail", mock.Anything, "new@example.com").Return(nil, nil)
	userRepo.On("UpdateProfile", mock.Anything, testUserID, "New Name", "new@example.com", (*string)(nil)).Return(nil)

	role := "admin"
	userRepo.On("GetByID", mock.Anything, testUserID).Return(&domain.User{
		ID:        testUserID,
		AccountID: testAccountID,
		Name:      "New Name",
		Email:     "new@example.com",
		Role:      &role,
		CreatedAt: time.Now(),
	}, nil)

	_, router := setupMeTestRouter(userRepo, accountRepo, jwtMock)

	body := map[string]string{
		"name":  "New Name",
		"email": "new@example.com",
	}
	w := performRequest(router, http.MethodPut, "/v1/me", body, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp domain.User
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "New Name", resp.Name)

	userRepo.AssertExpectations(t)
}

func TestChangePassword_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	hashedPw, _ := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	hashStr := string(hashedPw)
	role := "admin"

	userRepo.On("GetByID", mock.Anything, testUserID).Return(&domain.User{
		ID:           testUserID,
		AccountID:    testAccountID,
		PasswordHash: &hashStr,
		Role:         &role,
		CreatedAt:    time.Now(),
	}, nil)
	userRepo.On("UpdatePasswordHash", mock.Anything, testUserID, mock.AnythingOfType("string")).Return(nil)

	_, router := setupMeTestRouter(userRepo, accountRepo, jwtMock)

	body := map[string]string{
		"current_password": "old-password",
		"new_password":     "new-password",
	}
	w := performRequest(router, http.MethodPatch, "/v1/me/password", body, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "password updated", resp["message"])

	userRepo.AssertExpectations(t)
}

func TestChangePassword_WrongPassword(t *testing.T) {
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	hashedPw, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	hashStr := string(hashedPw)
	role := "admin"

	userRepo.On("GetByID", mock.Anything, testUserID).Return(&domain.User{
		ID:           testUserID,
		AccountID:    testAccountID,
		PasswordHash: &hashStr,
		Role:         &role,
		CreatedAt:    time.Now(),
	}, nil)

	_, router := setupMeTestRouter(userRepo, accountRepo, jwtMock)

	body := map[string]string{
		"current_password": "wrong-password",
		"new_password":     "new-password",
	}
	w := performRequest(router, http.MethodPatch, "/v1/me/password", body, testToken)

	// ChangePasswordUseCase returns ErrInvalidCredentials which respondErr maps to 400 (default)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "AUTH_INVALID_CREDENTIALS", resp["code"])
}

func TestGetOnboardingProgress_Success(t *testing.T) {
	// Verifies: GET /me/onboarding-progress returns the DTO produced
	// by the underlying use-case, including the derived
	// completion_percentage computed server-side.
	accountRepo := new(MockAccountRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	accountRepo.On("GetOnboardingProgress", mock.Anything, testAccountID).
		Return(&domain.OnboardingProgress{
			// Bot flags travel and count for nothing; the percentage measures the
			// SALES milestones, so this account is 2/5 on products + customers.
			AgentCreated:      true,
			PreviewChatTested: true,
			ChannelConnected:  false,
			ProductsAdded:     true,
			CustomersAdded:    true,
		}, nil)
	accountRepo.On("GetByID", mock.Anything, testAccountID).
		Return(&domain.Account{ID: testAccountID, Name: "Acme"}, nil)

	getProgressUC := usecases.NewGetOnboardingProgressUseCase(accountRepo)
	handler := NewMeHandler(nil, nil, nil, getProgressUC, nil, nil, nil)

	router := setupTestRouter()
	router.Use(setupAuthMiddleware(jwtMock))
	router.GET("/v1/me/onboarding-progress", handler.HandleGetOnboardingProgress)

	w := performRequest(router, http.MethodGet, "/v1/me/onboarding-progress", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["agent_created"])
	assert.Equal(t, true, resp["preview_chat_tested"])
	assert.Equal(t, false, resp["channel_connected"])
	assert.Equal(t, true, resp["products_added"])
	assert.Equal(t, true, resp["customers_added"])
	assert.Equal(t, false, resp["proposal_made"])
	// 2 of the 6 SALES milestones. The two bot flags above move nothing.
	assert.Equal(t, float64(33), resp["completion_percentage"])
	assert.Equal(t, "Acme", resp["account_name"])
}

func TestGetOnboardingProgress_NotFound(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	accountRepo.On("GetOnboardingProgress", mock.Anything, testAccountID).
		Return(nil, nil)

	getProgressUC := usecases.NewGetOnboardingProgressUseCase(accountRepo)
	handler := NewMeHandler(nil, nil, nil, getProgressUC, nil, nil, nil)

	router := setupTestRouter()
	router.Use(setupAuthMiddleware(jwtMock))
	router.GET("/v1/me/onboarding-progress", handler.HandleGetOnboardingProgress)

	w := performRequest(router, http.MethodGet, "/v1/me/onboarding-progress", nil, testToken)

	// respondErr delegates to the shared status table, so a missing account
	// is a 404 here like everywhere else (it used to default to 400).
	assert.Equal(t, http.StatusNotFound, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "ACCOUNT_NOT_FOUND", resp["code"])
}

func TestDismissOnboardingChecklist_Success(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	accountRepo.On("DismissOnboardingChecklist",
		mock.Anything, testAccountID, mock.Anything).Return(nil)

	dismissUC := usecases.NewDismissOnboardingChecklistUseCase(accountRepo)
	handler := NewMeHandler(nil, nil, nil, nil, dismissUC, nil, nil)

	router := setupTestRouter()
	router.Use(setupAuthMiddleware(jwtMock))
	router.POST("/v1/me/onboarding-checklist/dismiss", handler.HandleDismissOnboardingChecklist)

	w := performRequest(router, http.MethodPost, "/v1/me/onboarding-checklist/dismiss", nil, testToken)

	assert.Equal(t, http.StatusNoContent, w.Code)
	accountRepo.AssertExpectations(t)
}

// respondErr must map a missing-cipher server misconfig to 503, not the
// default 400 — saving a BYOK key with BYOK_ENCRYPTION_KEY unset is a
// deployment fault, and a 400 wrongly reads as the user's bad input.
func TestRespondErr_BYOKCipherUnconfiguredIs503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	respondErr(c, &domain.AppError{
		Code:    "BYOK_CIPHER_UNCONFIGURED",
		Message: "encryption key is not configured on this server",
	})

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var body domain.AppError
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "BYOK_CIPHER_UNCONFIGURED", body.Code)
}

// An unmapped code still defaults to 400 (regression guard).
func TestRespondErr_UnmappedCodeStays400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	respondErr(c, &domain.AppError{Code: "SOME_OTHER", Message: "x"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
