package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

func TestLogin_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	jwtService := new(MockJWTService)

	hashedPw, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	hashedStr := string(hashedPw)
	role := "admin"

	testUser := &domain.User{
		ID:              "user-1",
		AccountID:       "acc-1",
		Name:            "John",
		Email:           "john@example.com",
		PasswordHash:    &hashedStr,
		Role:            &role,
		IsEmailVerified: true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	userRepo.On("GetByEmail", mock.Anything, "john@example.com").Return(testUser, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(&domain.Account{ID: "acc-1", OnboardingCompleted: false}, nil)
	jwtService.On("GenerateToken", testUser).Return("jwt-token-123", nil)

	authenticateUC := usecases.NewAuthenticateUserUseCase(userRepo, accountRepo, jwtService)
	handler := NewAuthHandler(authenticateUC, nil, nil, nil, nil, nil, nil, nil)

	router := setupTestRouter()
	router.POST("/v1/auth/login", handler.Login)

	body := map[string]string{"email": "john@example.com", "password": "correct-password"}
	w := performRequest(router, http.MethodPost, "/v1/auth/login", body, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "jwt-token-123", resp["token"])
	assert.Equal(t, true, resp["is_new"]) // no agents = new user

	userRepo.AssertExpectations(t)
	accountRepo.AssertExpectations(t)
	jwtService.AssertExpectations(t)
}

func TestLogin_InvalidCredentials(t *testing.T) {
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	jwtService := new(MockJWTService)

	hashedPw, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	hashedStr := string(hashedPw)
	role := "admin"

	testUser := &domain.User{
		ID:              "user-1",
		AccountID:       "acc-1",
		Email:           "john@example.com",
		PasswordHash:    &hashedStr,
		Role:            &role,
		IsEmailVerified: true,
	}

	userRepo.On("GetByEmail", mock.Anything, "john@example.com").Return(testUser, nil)

	authenticateUC := usecases.NewAuthenticateUserUseCase(userRepo, accountRepo, jwtService)
	handler := NewAuthHandler(authenticateUC, nil, nil, nil, nil, nil, nil, nil)

	router := setupTestRouter()
	router.POST("/v1/auth/login", handler.Login)

	body := map[string]string{"email": "john@example.com", "password": "wrong-password"}
	w := performRequest(router, http.MethodPost, "/v1/auth/login", body, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "AUTH_INVALID_CREDENTIALS", resp["code"])
}

func TestLogin_UnverifiedEmailStillSucceeds(t *testing.T) {
	// Email verification is no longer a login gate — users with an
	// unverified email should still receive a valid session. The /me
	// endpoint exposes is_email_verified so the UI can nudge them.
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	jwtService := new(MockJWTService)

	hashedPw, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	hashedStr := string(hashedPw)
	role := "admin"

	testUser := &domain.User{
		ID:              "user-1",
		AccountID:       "acc-1",
		Email:           "john@example.com",
		PasswordHash:    &hashedStr,
		Role:            &role,
		IsEmailVerified: false,
	}

	userRepo.On("GetByEmail", mock.Anything, "john@example.com").Return(testUser, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(&domain.Account{
		ID:                  "acc-1",
		OnboardingCompleted: true,
	}, nil)
	jwtService.On("GenerateToken", testUser).Return("test-token", nil)

	authenticateUC := usecases.NewAuthenticateUserUseCase(userRepo, accountRepo, jwtService)
	handler := NewAuthHandler(authenticateUC, nil, nil, nil, nil, nil, nil, nil)

	router := setupTestRouter()
	router.POST("/v1/auth/login", handler.Login)

	body := map[string]string{"email": "john@example.com", "password": "correct-password"}
	w := performRequest(router, http.MethodPost, "/v1/auth/login", body, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "test-token", resp["token"])
}

func TestLogin_InvalidBody(t *testing.T) {
	authenticateUC := usecases.NewAuthenticateUserUseCase(new(MockUserRepo), new(MockAccountRepo), new(MockJWTService))
	handler := NewAuthHandler(authenticateUC, nil, nil, nil, nil, nil, nil, nil)

	router := setupTestRouter()
	router.POST("/v1/auth/login", handler.Login)

	// Send completely invalid JSON
	w := performRequest(router, http.MethodPost, "/v1/auth/login", "not-json{", "")

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestVerifyEmail_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)

	testUser := &domain.User{ID: "user-1", AccountID: "acc-1", Email: "john@example.com"}
	userRepo.On("GetByVerificationToken", mock.Anything, "valid-token").Return(testUser, nil)
	userRepo.On("VerifyEmail", mock.Anything, "user-1").Return(nil)
	emailSvc.On("SendWelcomeEmail", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	verifyEmailUC := usecases.NewVerifyEmailUseCase(userRepo, emailSvc, "http://localhost:8080")
	handler := NewAuthHandler(nil, nil, nil, nil, nil, nil, verifyEmailUC, nil)

	router := setupTestRouter()
	router.GET("/v1/auth/verify-email", handler.HandleVerifyEmail)

	w := performRequest(router, http.MethodGet, "/v1/auth/verify-email?token=valid-token", nil, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "email verified successfully", resp["message"])

	userRepo.AssertExpectations(t)
}

func TestVerifyEmail_MissingToken(t *testing.T) {
	verifyEmailUC := usecases.NewVerifyEmailUseCase(new(MockUserRepo), new(MockEmailService), "http://localhost:8080")
	handler := NewAuthHandler(nil, nil, nil, nil, nil, nil, verifyEmailUC, nil)

	router := setupTestRouter()
	router.GET("/v1/auth/verify-email", handler.HandleVerifyEmail)

	w := performRequest(router, http.MethodGet, "/v1/auth/verify-email", nil, "")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_REQUEST", resp["code"])
}

func TestForgotPassword_Returns200Always(t *testing.T) {
	userRepo := new(MockUserRepo)
	emailService := new(MockEmailService)

	// Simulate user not found -- should still return 200
	userRepo.On("GetByEmail", mock.Anything, "nonexistent@example.com").Return(nil, errors.New("not found"))

	forgotPasswordUC := usecases.NewForgotPasswordUseCase(userRepo, emailService, "https://app.example.com")
	handler := NewAuthHandler(nil, nil, forgotPasswordUC, nil, nil, nil, nil, nil)

	router := setupTestRouter()
	router.POST("/v1/auth/forgot-password", handler.ForgotPassword)

	body := map[string]string{"email": "nonexistent@example.com"}
	w := performRequest(router, http.MethodPost, "/v1/auth/forgot-password", body, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp["message"], "If the email exists")
}
