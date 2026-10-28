package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// newRLEngine builds a gin engine that optionally stamps an accountID into the
// context (simulating AuthMiddleware) before the rate limiter runs.
func newRLEngine(cfg RateLimitConfig, accountID string) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if accountID != "" {
			c.Set("accountID", accountID)
		}
		c.Next()
	})
	r.Use(PerAccountIPRateLimiter(cfg))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

// TestPerAccountIPRateLimiter_ThrottlesAfterBurst: with a burst of 2, the first
// two requests pass and the third is rejected with 429 + Retry-After.
func TestPerAccountIPRateLimiter_ThrottlesAfterBurst(t *testing.T) {
	r := newRLEngine(RateLimitConfig{RequestsPerMinute: 60, Burst: 2}, "acc_1")

	do := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
		return w
	}

	assert.Equal(t, http.StatusOK, do().Code, "1st request within burst")
	assert.Equal(t, http.StatusOK, do().Code, "2nd request within burst")

	third := do()
	assert.Equal(t, http.StatusTooManyRequests, third.Code, "3rd request throttled")

	retryAfter := third.Header().Get("Retry-After")
	assert.NotEmpty(t, retryAfter, "Retry-After header must be set on 429")
	secs, err := strconv.Atoi(retryAfter)
	assert.NoError(t, err, "Retry-After must be an integer seconds value")
	assert.GreaterOrEqual(t, secs, 1, "Retry-After must be >= 1s")
	assert.Contains(t, third.Body.String(), "RATE_LIMITED")
}

// TestPerAccountIPRateLimiter_IsolatesByAccount: each (account_id, ip) gets its
// own bucket, so one account exhausting its budget doesn't throttle another.
func TestPerAccountIPRateLimiter_IsolatesByAccount(t *testing.T) {
	cfg := RateLimitConfig{RequestsPerMinute: 60, Burst: 1}

	rA := newRLEngine(cfg, "acc_A")
	wA1 := httptest.NewRecorder()
	rA.ServeHTTP(wA1, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.Equal(t, http.StatusOK, wA1.Code)
	wA2 := httptest.NewRecorder()
	rA.ServeHTTP(wA2, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.Equal(t, http.StatusTooManyRequests, wA2.Code, "same account throttled after burst")

	// A different account on a fresh limiter still gets its full burst.
	rB := newRLEngine(cfg, "acc_B")
	wB := httptest.NewRecorder()
	rB.ServeHTTP(wB, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.Equal(t, http.StatusOK, wB.Code, "distinct account not affected")
}

// TestPerAccountIPRateLimiter_DefaultsApplied: an empty config yields the
// documented 100 req/min default and tolerates a 100-request burst.
func TestPerAccountIPRateLimiter_DefaultsApplied(t *testing.T) {
	r := newRLEngine(RateLimitConfig{}, "acc_default")
	for i := 0; i < DefaultRateLimitPerMinute; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
		assert.Equal(t, http.StatusOK, w.Code, "request %d within default burst", i+1)
	}
	// 101st exceeds the default burst.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

// newConvRLEngine builds a gin engine exposing POST /conversations/:id/x guarded
// by the per-conversation limiter, so the bucket key is the `:id` path param.
func newConvRLEngine(cfg RateLimitConfig) *gin.Engine {
	r := gin.New()
	r.POST("/conversations/:id/x", PerConversationRateLimiter(cfg), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

// TestPerConversationRateLimiter_ThrottlesPerConversation: a burst of 2 lets the
// first two messages into a conversation through; the third is 429 with a valid
// Retry-After header.
func TestPerConversationRateLimiter_ThrottlesPerConversation(t *testing.T) {
	r := newConvRLEngine(RateLimitConfig{RequestsPerMinute: 60, Burst: 2})

	do := func(convID string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/conversations/"+convID+"/x", nil))
		return w
	}

	assert.Equal(t, http.StatusOK, do("conv_1").Code, "1st message within burst")
	assert.Equal(t, http.StatusOK, do("conv_1").Code, "2nd message within burst")

	third := do("conv_1")
	assert.Equal(t, http.StatusTooManyRequests, third.Code, "3rd message throttled")
	secs, err := strconv.Atoi(third.Header().Get("Retry-After"))
	assert.NoError(t, err, "Retry-After must be an integer seconds value")
	assert.GreaterOrEqual(t, secs, 1, "Retry-After must be >= 1s")
	assert.Contains(t, third.Body.String(), "RATE_LIMITED")
}

// TestPerConversationRateLimiter_IsolatesByConversation: each conversation id
// has its own bucket, so spamming one thread doesn't throttle another.
func TestPerConversationRateLimiter_IsolatesByConversation(t *testing.T) {
	r := newConvRLEngine(RateLimitConfig{RequestsPerMinute: 60, Burst: 1})

	do := func(convID string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/conversations/"+convID+"/x", nil))
		return w
	}

	assert.Equal(t, http.StatusOK, do("conv_A").Code, "first message to A passes")
	assert.Equal(t, http.StatusTooManyRequests, do("conv_A").Code, "A throttled after burst")
	// A distinct conversation still gets its full burst.
	assert.Equal(t, http.StatusOK, do("conv_B").Code, "distinct conversation not affected")
}

// newLoginRLEngine mounts LoginRateLimiter in front of a handler that echoes
// the body it received, so the tests can assert both throttling AND that the
// middleware left the request body intact for the real login handler.
func newLoginRLEngine(cfg RateLimitConfig) *gin.Engine {
	r := gin.New()
	r.Use(LoginRateLimiter(cfg))
	r.POST("/login", func(c *gin.Context) {
		var body struct {
			Email string `json:"email"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"err": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"email": body.Email})
	})
	return r
}

func postLogin(r *gin.Engine, email, ip string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"email":"`+email+`","password":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = ip + ":12345"
	r.ServeHTTP(w, req)
	return w
}

// US-F012: repeated attempts against the SAME account from the same IP are
// throttled once the burst is spent, with a Retry-After the client can honour.
func TestLoginRateLimiter_ThrottlesRepeatedAttemptsOnSameEmail(t *testing.T) {
	r := newLoginRLEngine(RateLimitConfig{RequestsPerMinute: 5, Burst: 2})

	assert.Equal(t, http.StatusOK, postLogin(r, "victim@acme.com", "203.0.113.10").Code)
	assert.Equal(t, http.StatusOK, postLogin(r, "victim@acme.com", "203.0.113.10").Code)

	third := postLogin(r, "victim@acme.com", "203.0.113.10")
	assert.Equal(t, http.StatusTooManyRequests, third.Code)
	assert.NotEmpty(t, third.Header().Get("Retry-After"))
	assert.Contains(t, third.Body.String(), "RATE_LIMITED")
}

// The middleware must not eat the body — the login handler still needs it.
func TestLoginRateLimiter_PreservesRequestBody(t *testing.T) {
	r := newLoginRLEngine(RateLimitConfig{RequestsPerMinute: 60, Burst: 5})

	w := postLogin(r, "user@acme.com", "203.0.113.20")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "user@acme.com")
}

// Distinct targets get distinct buckets: spraying one account must not lock
// out a different user coming from the same office IP.
func TestLoginRateLimiter_IsolatesByEmail(t *testing.T) {
	r := newLoginRLEngine(RateLimitConfig{RequestsPerMinute: 5, Burst: 1})

	assert.Equal(t, http.StatusOK, postLogin(r, "a@acme.com", "203.0.113.30").Code)
	assert.Equal(t, http.StatusTooManyRequests, postLogin(r, "a@acme.com", "203.0.113.30").Code)
	// Same IP, different account → own budget.
	assert.Equal(t, http.StatusOK, postLogin(r, "b@acme.com", "203.0.113.30").Code)
}

// ...and distinct IPs get distinct buckets for the same email, so the
// per-IP limiter in front stays the defence against a distributed spray
// while this one caps each source.
func TestLoginRateLimiter_IsolatesByIP(t *testing.T) {
	r := newLoginRLEngine(RateLimitConfig{RequestsPerMinute: 5, Burst: 1})

	assert.Equal(t, http.StatusOK, postLogin(r, "a@acme.com", "203.0.113.40").Code)
	assert.Equal(t, http.StatusTooManyRequests, postLogin(r, "a@acme.com", "203.0.113.40").Code)
	assert.Equal(t, http.StatusOK, postLogin(r, "a@acme.com", "203.0.113.41").Code)
}

// Case and whitespace must not mint a fresh budget — "Victim@ACME.com " is
// the same target as "victim@acme.com".
func TestLoginRateLimiter_NormalizesEmail(t *testing.T) {
	r := newLoginRLEngine(RateLimitConfig{RequestsPerMinute: 5, Burst: 1})

	assert.Equal(t, http.StatusOK, postLogin(r, "victim@acme.com", "203.0.113.50").Code)
	assert.Equal(t, http.StatusTooManyRequests,
		postLogin(r, "Victim@ACME.com", "203.0.113.50").Code)
}

// A body with no email (or garbage) still consumes the IP bucket — never a
// free pass around the limiter.
func TestLoginRateLimiter_LimitsBodylessAttempts(t *testing.T) {
	r := newLoginRLEngine(RateLimitConfig{RequestsPerMinute: 5, Burst: 1})

	do := func() int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`not json`))
		req.RemoteAddr = "203.0.113.60:1234"
		r.ServeHTTP(w, req)
		return w.Code
	}
	assert.NotEqual(t, http.StatusTooManyRequests, do())
	assert.Equal(t, http.StatusTooManyRequests, do())
}

// The bucket refills: after the wait implied by the rate, the caller is let
// back in (throttle, not permanent lockout).
func TestLoginRateLimiter_BucketRefills(t *testing.T) {
	// 6000/min = 100/s → a token every 10ms.
	r := newLoginRLEngine(RateLimitConfig{RequestsPerMinute: 6000, Burst: 1})

	assert.Equal(t, http.StatusOK, postLogin(r, "u@acme.com", "203.0.113.70").Code)
	assert.Equal(t, http.StatusTooManyRequests, postLogin(r, "u@acme.com", "203.0.113.70").Code)
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, http.StatusOK, postLogin(r, "u@acme.com", "203.0.113.70").Code)
}
