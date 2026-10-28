package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTokenBlacklist implements domain.TokenBlacklist for testing.
type MockTokenBlacklist struct{ mock.Mock }

func (m *MockTokenBlacklist) Revoke(ctx context.Context, tokenID string, ttl time.Duration) error {
	return m.Called(ctx, tokenID, ttl).Error(0)
}
func (m *MockTokenBlacklist) IsRevoked(ctx context.Context, tokenID string) bool {
	return m.Called(ctx, tokenID).Bool(0)
}

func TestLogout_ValidToken(t *testing.T) {
	jwtMock := new(MockJWTService)
	blacklist := new(MockTokenBlacklist)

	claims := &domain.UserClaims{
		UserID:    testUserID,
		AccountID: testAccountID,
		Role:      "admin",
	}
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(1 * time.Hour))

	jwtMock.On("ValidateToken", testToken).Return(claims, nil)
	blacklist.On("Revoke", mock.Anything, testToken, mock.MatchedBy(func(d time.Duration) bool {
		return d > 0 && d <= 1*time.Hour
	})).Return(nil)

	handler := NewLogoutHandler(blacklist, jwtMock)

	router := setupTestRouter()
	authMw := middleware.AuthMiddleware(jwtMock)
	router.POST("/v1/auth/logout", authMw, handler.HandleLogout)

	w := performRequest(router, http.MethodPost, "/v1/auth/logout", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "token revoked", resp["message"])

	blacklist.AssertExpectations(t)
}

func TestLogout_ExpiredToken(t *testing.T) {
	jwtMock := new(MockJWTService)
	blacklist := new(MockTokenBlacklist)

	// First call from middleware succeeds (validates token)
	claims := &domain.UserClaims{
		UserID:    testUserID,
		AccountID: testAccountID,
		Role:      "admin",
	}
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(1 * time.Hour))
	jwtMock.On("ValidateToken", "about-to-expire").Return(claims, nil).Once()

	// Second call from handler returns error (token expired between middleware and handler)
	jwtMock.On("ValidateToken", "about-to-expire").Return(nil, errors.New("token expired")).Once()

	handler := NewLogoutHandler(blacklist, jwtMock)

	router := setupTestRouter()
	authMw := middleware.AuthMiddleware(jwtMock)
	router.POST("/v1/auth/logout", authMw, handler.HandleLogout)

	w := performRequest(router, http.MethodPost, "/v1/auth/logout", nil, "about-to-expire")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "token already expired", resp["message"])
}

func TestLogout_BlacklistRevokeFails(t *testing.T) {
	jwtMock := new(MockJWTService)
	blacklist := new(MockTokenBlacklist)

	claims := &domain.UserClaims{
		UserID:    testUserID,
		AccountID: testAccountID,
		Role:      "admin",
	}
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(1 * time.Hour))

	jwtMock.On("ValidateToken", testToken).Return(claims, nil)
	blacklist.On("Revoke", mock.Anything, testToken, mock.Anything).Return(errors.New("redis unavailable"))

	handler := NewLogoutHandler(blacklist, jwtMock)

	router := setupTestRouter()
	authMw := middleware.AuthMiddleware(jwtMock)
	router.POST("/v1/auth/logout", authMw, handler.HandleLogout)

	w := performRequest(router, http.MethodPost, "/v1/auth/logout", nil, testToken)

	// Should still return 200 (best-effort)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "token revoked (best-effort)", resp["message"])
}

func TestLogout_NoAuth(t *testing.T) {
	jwtMock := new(MockJWTService)
	blacklist := new(MockTokenBlacklist)

	handler := NewLogoutHandler(blacklist, jwtMock)

	router := setupTestRouter()
	authMw := middleware.AuthMiddleware(jwtMock)
	router.POST("/v1/auth/logout", authMw, handler.HandleLogout)

	// No token provided
	w := performRequest(router, http.MethodPost, "/v1/auth/logout", nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
