package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

// ── /metrics bearer token ────────────────────────────────────────────────

func TestMetricsAuth_RequiresBearerWhenConfigured(t *testing.T) {
	r := gin.New()
	r.GET("/metrics", MetricsAuth("s3cret"), func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	for name, tc := range map[string]struct {
		header string
		want   int
	}{
		"missing":     {"", http.StatusUnauthorized},
		"wrong":       {"Bearer nope", http.StatusUnauthorized},
		"right":       {"Bearer s3cret", http.StatusOK},
		"no-scheme":   {"s3cret", http.StatusOK},
		"query-token": {"", http.StatusUnauthorized},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics?token=s3cret", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, tc.want, w.Code)
		})
	}
}

func TestMetricsAuth_EmptyTokenIsPassThrough(t *testing.T) {
	r := gin.New()
	r.GET("/metrics", MetricsAuth(""), func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

// ── ?token= only on the WebSocket handshake ──────────────────────────────

func TestAuthMiddleware_QueryTokenRefusedOutsideWebSocket(t *testing.T) {
	jwtSvc := new(MockJWTService)
	jwtSvc.On("ValidateToken", "ws-token").Return(&domain.UserClaims{
		UserID: "u", AccountID: "a", Role: "admin",
	}, nil).Maybe()

	r := gin.New()
	r.GET("/v1/plans", AuthMiddleware(jwtSvc), func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/v1/ws/chat", AuthMiddleware(jwtSvc), func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/plans?token=ws-token", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code, "a REST route must not authenticate from the query string")

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/ws/chat?token=ws-token", nil))
	assert.Equal(t, http.StatusOK, w.Code, "the WebSocket handshake still may")
}

// ── session invalidation (member removal) ────────────────────────────────

type stampReader struct {
	at    *time.Time
	calls int
}

func (s *stampReader) SessionInvalidatedAt(context.Context, string) (*time.Time, error) {
	s.calls++
	return s.at, nil
}

func sessionRouter(reader domain.SessionInvalidationReader, jwtSvc *MockJWTService) *gin.Engine {
	r := gin.New()
	r.GET("/me", AuthMiddleware(jwtSvc), SessionInvalidationGuard(reader), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

func TestSessionInvalidationGuard_RefusesTokenIssuedBeforeRemoval(t *testing.T) {
	removedAt := time.Now().Add(-time.Minute)
	jwtSvc := new(MockJWTService)
	jwtSvc.On("ValidateToken", "old").Return(&domain.UserClaims{
		UserID: "u1", AccountID: "a1", Role: "attendant",
		RegisteredClaims: jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(removedAt.Add(-time.Hour))},
	}, nil)
	jwtSvc.On("ValidateToken", "new").Return(&domain.UserClaims{
		UserID: "u1", AccountID: "a1", Role: "attendant",
		RegisteredClaims: jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(time.Now())},
	}, nil)

	reader := &stampReader{at: &removedAt}
	r := sessionRouter(reader, jwtSvc)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer old")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "AUTH_SESSION_REVOKED")

	// A token minted after the removal (re-invited member) is fine, and the
	// stamp was cached — one lookup for two requests.
	req = httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer new")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, reader.calls)
}

func TestSessionInvalidationGuard_NilReaderAndNoStampPassThrough(t *testing.T) {
	jwtSvc := new(MockJWTService)
	jwtSvc.On("ValidateToken", "t").Return(&domain.UserClaims{
		UserID: "u1", AccountID: "a1", Role: "admin",
		RegisteredClaims: jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(time.Now())},
	}, nil)

	for _, reader := range []domain.SessionInvalidationReader{nil, &stampReader{}} {
		r := sessionRouter(reader, jwtSvc)
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", "Bearer t")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}

func TestSessionInvalidatedBefore(t *testing.T) {
	stamp := time.Date(2026, 9, 1, 12, 0, 0, 500, time.UTC)
	assert.True(t, domain.SessionInvalidatedBefore(stamp.Add(-time.Hour), &stamp))
	assert.True(t, domain.SessionInvalidatedBefore(stamp, &stamp), "same second: the removal wins")
	assert.False(t, domain.SessionInvalidatedBefore(stamp.Add(time.Second), &stamp))
	assert.False(t, domain.SessionInvalidatedBefore(stamp, nil))
	assert.False(t, domain.SessionInvalidatedBefore(time.Time{}, &stamp))
}
