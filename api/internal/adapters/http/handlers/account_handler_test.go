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

func TestRegister_Success(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	userRepo := new(MockUserRepo)
	usageRepo := new(MockUsageRepo)
	txManager := new(MockTransactionManager)
	emailService := new(MockEmailService)

	// GetByEmail returns nil => email not taken
	userRepo.On("GetByEmail", mock.Anything, "jane@example.com").Return(nil, nil)
	accountRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Account")).Return(nil)
	userRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.User")).Return(nil)
	userRepo.On("SetVerificationToken", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)
	usageRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AccountUsage")).Return(nil)
	emailService.On("SendVerificationEmail", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)

	createUC := usecases.NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailService, "http://test")
	handler := NewAccountHandler(createUC, nil, nil)

	router := setupTestRouter()
	router.POST("/v1/accounts", handler.HandleCreateAccount)

	body := map[string]interface{}{
		"account_name":   "Test Co",
		"admin_name":     "Jane",
		"admin_email":    "jane@example.com",
		"password":       "securepass",
		"terms_accepted": true,
	}
	w := performRequest(router, http.MethodPost, "/v1/accounts", body, "")

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "account created successfully", resp["message"])

	userRepo.AssertExpectations(t)
	accountRepo.AssertExpectations(t)
	usageRepo.AssertExpectations(t)
}

func TestRegister_EmailAlreadyExists(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	userRepo := new(MockUserRepo)
	usageRepo := new(MockUsageRepo)
	txManager := new(MockTransactionManager)
	emailService := new(MockEmailService)

	role := "admin"
	userRepo.On("GetByEmail", mock.Anything, "taken@example.com").Return(&domain.User{
		ID:        "existing-user",
		AccountID: "existing-acc",
		Email:     "taken@example.com",
		Role:      &role,
	}, nil)

	createUC := usecases.NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailService, "http://test")
	handler := NewAccountHandler(createUC, nil, nil)

	router := setupTestRouter()
	router.POST("/v1/accounts", handler.HandleCreateAccount)

	body := map[string]interface{}{
		"account_name":   "Test Co",
		"admin_name":     "Jane",
		"admin_email":    "taken@example.com",
		"password":       "securepass",
		"terms_accepted": true,
	}
	w := performRequest(router, http.MethodPost, "/v1/accounts", body, "")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "AUTH_EMAIL_ALREADY_EXISTS", resp["code"])
}

func TestRegister_InvalidBody(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	userRepo := new(MockUserRepo)
	usageRepo := new(MockUsageRepo)
	txManager := new(MockTransactionManager)
	emailService := new(MockEmailService)

	createUC := usecases.NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailService, "http://test")
	handler := NewAccountHandler(createUC, nil, nil)

	router := setupTestRouter()
	router.POST("/v1/accounts", handler.HandleCreateAccount)

	// Empty body -> missing required fields -> 400
	w := performRequest(router, http.MethodPost, "/v1/accounts", map[string]string{}, "")

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteAccount_Success(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	userRepo := new(MockUserRepo)
	jwtMock := new(MockJWTService)

	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	role := "admin"
	userRepo.On("GetByID", mock.Anything, testUserID).Return(&domain.User{
		ID:        testUserID,
		AccountID: testAccountID,
		Role:      &role,
		CreatedAt: time.Now(),
	}, nil)
	accountRepo.On("Delete", mock.Anything, testAccountID).Return(nil)

	deleteUC := usecases.NewDeleteAccountUseCase(accountRepo, userRepo)
	handler := NewAccountHandler(nil, deleteUC, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.DELETE("/v1/accounts", authMw, middleware.RequireRole("admin"), handler.HandleDeleteAccount)

	w := performRequest(router, http.MethodDelete, "/v1/accounts", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "account deleted", resp["message"])

	userRepo.AssertExpectations(t)
	accountRepo.AssertExpectations(t)
}

func TestDeleteAccount_AttendantDenied(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "attendant")

	handler := NewAccountHandler(nil, nil, nil)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	router.DELETE("/v1/accounts", authMw, middleware.RequireRole("admin"), handler.HandleDeleteAccount)

	w := performRequest(router, http.MethodDelete, "/v1/accounts", nil, testToken)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "FORBIDDEN", resp["code"])
}
