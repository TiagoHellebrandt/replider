package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() { gin.SetMode(gin.TestMode) }

// TestRateLimiter_SignupCapPerIP exercises the exact limit wired onto the public
// signup endpoint (POST /accounts, US-053): 5 requests per hour per IP. The
// first five from one IP pass; the sixth is rejected with 429.
func TestRateLimiter_SignupCapPerIP(t *testing.T) {
	r := gin.New()
	r.POST("/accounts", RateLimiter("5-H"), func(c *gin.Context) { c.Status(http.StatusOK) })

	do := func(ip string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/accounts", nil)
		req.RemoteAddr = ip + ":54321"
		r.ServeHTTP(w, req)
		return w
	}

	for i := 1; i <= 5; i++ {
		assert.Equal(t, http.StatusOK, do("203.0.113.7").Code, "signup %d within hourly cap", i)
	}
	assert.Equal(t, http.StatusTooManyRequests, do("203.0.113.7").Code, "6th signup from same IP throttled")
}

// TestRateLimiter_IsolatesByIP: each client IP gets its own hourly window, so one
// abusive IP exhausting its budget doesn't lock out a different legitimate IP.
func TestRateLimiter_IsolatesByIP(t *testing.T) {
	r := gin.New()
	r.POST("/accounts", RateLimiter("5-H"), func(c *gin.Context) { c.Status(http.StatusOK) })

	do := func(ip string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/accounts", nil)
		req.RemoteAddr = ip + ":54321"
		r.ServeHTTP(w, req)
		return w
	}

	// Exhaust the budget for one IP.
	for i := 1; i <= 5; i++ {
		do("198.51.100.1")
	}
	assert.Equal(t, http.StatusTooManyRequests, do("198.51.100.1").Code, "abusive IP throttled")

	// A different IP still gets its full allowance.
	assert.Equal(t, http.StatusOK, do("198.51.100.99").Code, "distinct IP not affected")
}
