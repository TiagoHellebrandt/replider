package middleware

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockJWTService implements domain.JWTService for testing.
type MockJWTService struct {
	mock.Mock
}

func (m *MockJWTService) GenerateToken(user *domain.User) (string, error) {
	args := m.Called(user)
	return args.String(0), args.Error(1)
}

func (m *MockJWTService) ValidateToken(token string) (*domain.UserClaims, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserClaims), args.Error(1)
}

func (m *MockJWTService) GenerateImpersonationToken(claims domain.ImpersonationClaims) (string, error) {
	args := m.Called(claims)
	return args.String(0), args.Error(1)
}

func (m *MockJWTService) GenerateMfaLoginToken(userID string) (string, error) {
	args := m.Called(userID)
	return args.String(0), args.Error(1)
}

func (m *MockJWTService) ValidateMfaLoginToken(token string) (*domain.MfaLoginClaims, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MfaLoginClaims), args.Error(1)
}

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAuthMiddleware_ValidBearerToken(t *testing.T) {
	jwtSvc := new(MockJWTService)
	claims := &domain.UserClaims{
		UserID:    "user_123",
		UserName:  "John Doe",
		AccountID: "acc_456",
		Role:      "admin",
	}
	jwtSvc.On("ValidateToken", "valid-token").Return(claims, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c.Request.Header.Set("Authorization", "Bearer valid-token")

	var nextCalled bool
	handler := AuthMiddleware(jwtSvc)

	// We need to use a full engine to test c.Next() properly,
	// but we can also just call the handler and check context values.
	r := gin.New()
	r.Use(handler)
	r.GET("/test", func(c *gin.Context) {
		nextCalled = true
		assert.Equal(t, "user_123", c.GetString("userID"))
		assert.Equal(t, "John Doe", c.GetString("userName"))
		assert.Equal(t, "acc_456", c.GetString("accountID"))
		assert.Equal(t, "admin", c.GetString("role"))
		c.Status(http.StatusOK)
	})

	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	r.ServeHTTP(w, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
	jwtSvc.AssertExpectations(t)
}

func TestAuthMiddleware_MissingAuthorizationHeader(t *testing.T) {
	jwtSvc := new(MockJWTService)

	r := gin.New()
	r.Use(AuthMiddleware(jwtSvc))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Authorization required")
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	jwtSvc := new(MockJWTService)
	jwtSvc.On("ValidateToken", "bad-token").Return(nil, errors.New("invalid token"))

	r := gin.New()
	r.Use(AuthMiddleware(jwtSvc))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid or expired token")
	jwtSvc.AssertExpectations(t)
}

func TestAuthMiddleware_TokenFromQueryParam(t *testing.T) {
	jwtSvc := new(MockJWTService)
	claims := &domain.UserClaims{
		UserID:    "user_ws",
		UserName:  "WS User",
		AccountID: "acc_ws",
		Role:      "attendant",
	}
	jwtSvc.On("ValidateToken", "ws-token").Return(claims, nil)

	var nextCalled bool
	r := gin.New()
	r.Use(AuthMiddleware(jwtSvc))
	r.GET("/ws/chat", func(c *gin.Context) {
		nextCalled = true
		assert.Equal(t, "user_ws", c.GetString("userID"))
		assert.Equal(t, "WS User", c.GetString("userName"))
		assert.Equal(t, "acc_ws", c.GetString("accountID"))
		assert.Equal(t, "attendant", c.GetString("role"))
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws/chat?token=ws-token", nil)
	r.ServeHTTP(w, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
	jwtSvc.AssertExpectations(t)
}

func TestAuthMiddleware_MalformedAuthorizationHeader(t *testing.T) {
	jwtSvc := new(MockJWTService)

	r := gin.New()
	r.Use(AuthMiddleware(jwtSvc))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "NotBearer some-token")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Authorization required")
}

// MockTokenBlacklist implements domain.TokenBlacklist for testing.
type MockTokenBlacklist struct {
	mock.Mock
}

func (m *MockTokenBlacklist) Revoke(ctx context.Context, tokenID string, ttl time.Duration) error {
	return m.Called(ctx, tokenID, ttl).Error(0)
}
func (m *MockTokenBlacklist) IsRevoked(ctx context.Context, tokenID string) bool {
	return m.Called(ctx, tokenID).Bool(0)
}

func TestAuthMiddleware_RevokedToken(t *testing.T) {
	jwtSvc := new(MockJWTService)
	blacklist := new(MockTokenBlacklist)

	claims := &domain.UserClaims{
		UserID:    "user_123",
		UserName:  "John Doe",
		AccountID: "acc_456",
		Role:      "admin",
	}
	jwtSvc.On("ValidateToken", "revoked-token").Return(claims, nil)
	blacklist.On("IsRevoked", mock.Anything, "revoked-token").Return(true)

	r := gin.New()
	r.Use(AuthMiddleware(jwtSvc, blacklist))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer revoked-token")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Token has been revoked")
	jwtSvc.AssertExpectations(t)
	blacklist.AssertExpectations(t)
}

func TestAuthMiddleware_ValidTokenWithBlacklist(t *testing.T) {
	jwtSvc := new(MockJWTService)
	blacklist := new(MockTokenBlacklist)

	claims := &domain.UserClaims{
		UserID:    "user_123",
		UserName:  "John Doe",
		AccountID: "acc_456",
		Role:      "admin",
	}
	jwtSvc.On("ValidateToken", "good-token").Return(claims, nil)
	blacklist.On("IsRevoked", mock.Anything, "good-token").Return(false)

	var nextCalled bool
	r := gin.New()
	r.Use(AuthMiddleware(jwtSvc, blacklist))
	r.GET("/test", func(c *gin.Context) {
		nextCalled = true
		assert.Equal(t, "user_123", c.GetString("userID"))
		assert.Equal(t, "good-token", c.GetString("token"))
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	r.ServeHTTP(w, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
	jwtSvc.AssertExpectations(t)
	blacklist.AssertExpectations(t)
}

func TestAuthMiddleware_NilBlacklistSkipsCheck(t *testing.T) {
	jwtSvc := new(MockJWTService)

	claims := &domain.UserClaims{
		UserID:    "user_123",
		UserName:  "John Doe",
		AccountID: "acc_456",
		Role:      "admin",
	}
	jwtSvc.On("ValidateToken", "valid-token").Return(claims, nil)

	var nextCalled bool
	r := gin.New()
	// No blacklist passed
	r.Use(AuthMiddleware(jwtSvc))
	r.GET("/test", func(c *gin.Context) {
		nextCalled = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	r.ServeHTTP(w, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAuthMiddleware_EmptyAccountIDClaim guards US-004: a token whose
// signature validates but whose AccountID claim is empty must be rejected
// with 401, so a forged/zero-claim token cannot reach a handler that would
// then run "WHERE account_id = ”" queries.
func TestAuthMiddleware_EmptyAccountIDClaim(t *testing.T) {
	jwtSvc := new(MockJWTService)
	claims := &domain.UserClaims{
		UserID:    "user_123",
		UserName:  "John Doe",
		AccountID: "", // forged / unset
		Role:      "admin",
	}
	jwtSvc.On("ValidateToken", "empty-account-token").Return(claims, nil)

	var nextCalled bool
	r := gin.New()
	r.Use(AuthMiddleware(jwtSvc))
	r.GET("/test", func(c *gin.Context) {
		nextCalled = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer empty-account-token")
	r.ServeHTTP(w, req)

	assert.False(t, nextCalled, "handler must not be reached when accountID is empty")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid or expired token")
	jwtSvc.AssertExpectations(t)
}

// TestAuthMiddleware_EmptyUserIDClaim mirrors the AccountID guard for UserID:
// any tenant-scope claim being empty must be treated as an invalid token.
func TestAuthMiddleware_EmptyUserIDClaim(t *testing.T) {
	jwtSvc := new(MockJWTService)
	claims := &domain.UserClaims{
		UserID:    "",
		UserName:  "John Doe",
		AccountID: "acc_456",
		Role:      "admin",
	}
	jwtSvc.On("ValidateToken", "empty-user-token").Return(claims, nil)

	var nextCalled bool
	r := gin.New()
	r.Use(AuthMiddleware(jwtSvc))
	r.GET("/test", func(c *gin.Context) {
		nextCalled = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer empty-user-token")
	r.ServeHTTP(w, req)

	assert.False(t, nextCalled, "handler must not be reached when userID is empty")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	jwtSvc.AssertExpectations(t)
}

// --- US-193: WebSocket auth via Authorization header (post-handshake) ---

// TestOptionalAuthMiddleware_AuthorizationHeaderFlow verifies the preferred WS
// auth path: a Bearer header on the upgrade request authenticates the user and
// does NOT emit the deprecation warning reserved for the query-param fallback.
func TestOptionalAuthMiddleware_AuthorizationHeaderFlow(t *testing.T) {
	jwtSvc := new(MockJWTService)
	claims := &domain.UserClaims{
		UserID:    "user_ws",
		UserName:  "WS User",
		AccountID: "acc_ws",
		Role:      "attendant",
	}
	jwtSvc.On("ValidateToken", "header-token").Return(claims, nil)

	var logBuf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(prev)

	var nextCalled bool
	r := gin.New()
	r.Use(OptionalAuthMiddleware(AuthMiddleware(jwtSvc)))
	r.GET("/ws/chat", func(c *gin.Context) {
		nextCalled = true
		assert.Equal(t, "user_ws", c.GetString("userID"))
		assert.Equal(t, "acc_ws", c.GetString("accountID"))
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws/chat", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	r.ServeHTTP(w, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, logBuf.String(), "deprecated WebSocket auth",
		"Authorization-header flow must not emit the deprecation warning")
	jwtSvc.AssertExpectations(t)
}

// TestOptionalAuthMiddleware_QueryTokenStillWorksWithWarning verifies the legacy
// ?token= path continues to authenticate but logs a deprecation warning.
func TestOptionalAuthMiddleware_QueryTokenStillWorksWithWarning(t *testing.T) {
	jwtSvc := new(MockJWTService)
	claims := &domain.UserClaims{
		UserID:    "user_ws",
		UserName:  "WS User",
		AccountID: "acc_ws",
		Role:      "attendant",
	}
	jwtSvc.On("ValidateToken", "query-token").Return(claims, nil)

	var logBuf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(prev)

	var nextCalled bool
	r := gin.New()
	r.Use(OptionalAuthMiddleware(AuthMiddleware(jwtSvc)))
	r.GET("/ws/chat", func(c *gin.Context) {
		nextCalled = true
		assert.Equal(t, "user_ws", c.GetString("userID"))
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws/chat?token=query-token", nil)
	r.ServeHTTP(w, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, logBuf.String(), "deprecated WebSocket auth",
		"query-param token flow must emit a deprecation warning")
	jwtSvc.AssertExpectations(t)
}

// TestOptionalAuthMiddleware_NoTokenAllowsFirstFrameAuth verifies that with no
// token present the request proceeds unauthenticated (for first-frame auth) and
// no deprecation warning is logged.
func TestOptionalAuthMiddleware_NoTokenAllowsFirstFrameAuth(t *testing.T) {
	jwtSvc := new(MockJWTService)

	var logBuf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(prev)

	var nextCalled bool
	r := gin.New()
	r.Use(OptionalAuthMiddleware(AuthMiddleware(jwtSvc)))
	r.GET("/ws/chat", func(c *gin.Context) {
		nextCalled = true
		assert.Empty(t, c.GetString("accountID"), "no token means no auth context")
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws/chat", nil)
	r.ServeHTTP(w, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, logBuf.String(), "deprecated WebSocket auth")
	// ValidateToken must never be called when no token is present.
	jwtSvc.AssertNotCalled(t, "ValidateToken", mock.Anything)
}

func TestAuthMiddleware_BearerHeaderTakesPrecedenceOverQuery(t *testing.T) {
	jwtSvc := new(MockJWTService)
	claims := &domain.UserClaims{
		UserID:    "header_user",
		UserName:  "Header User",
		AccountID: "acc_header",
		Role:      "admin",
	}
	jwtSvc.On("ValidateToken", "header-token").Return(claims, nil)

	var nextCalled bool
	r := gin.New()
	r.Use(AuthMiddleware(jwtSvc))
	r.GET("/test", func(c *gin.Context) {
		nextCalled = true
		assert.Equal(t, "header_user", c.GetString("userID"))
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test?token=query-token", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	r.ServeHTTP(w, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
	jwtSvc.AssertExpectations(t)
}
