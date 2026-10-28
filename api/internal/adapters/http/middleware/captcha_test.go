package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// stubVerifier records calls and returns canned outcomes for SignupCaptchaGuard
// tests so we never touch the real Cloudflare endpoint.
type stubVerifier struct {
	ok       bool
	err      error
	calls    int
	gotToken string
}

func (s *stubVerifier) Verify(_ context.Context, token, _ string) (bool, error) {
	s.calls++
	s.gotToken = token
	return s.ok, s.err
}

func newCaptchaEngine(mw gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/accounts", mw, func(c *gin.Context) { c.Status(http.StatusCreated) })
	return r
}

// doSignup fires one signup request from the given source IP, optionally
// carrying a captcha token header.
func doSignup(r *gin.Engine, ip, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/accounts", nil)
	req.RemoteAddr = ip + ":12345"
	if token != "" {
		req.Header.Set(captchaTokenHeader, token)
	}
	r.ServeHTTP(w, req)
	return w
}

// TestSignupCaptchaGuard_FreeAttemptsThenChallenge: the first 3 attempts from an
// IP pass without a token; the 4th requires one and is rejected when absent.
func TestSignupCaptchaGuard_FreeAttemptsThenChallenge(t *testing.T) {
	v := &stubVerifier{ok: true}
	r := newCaptchaEngine(SignupCaptchaGuard(v, CaptchaConfig{FreeAttempts: 3}))

	for i := 1; i <= 3; i++ {
		w := doSignup(r, "1.2.3.4", "")
		assert.Equal(t, http.StatusCreated, w.Code, "attempt %d should be free", i)
	}
	assert.Equal(t, 0, v.calls, "verifier must not be called within free quota")

	// 4th attempt, no token → challenged and rejected.
	w := doSignup(r, "1.2.3.4", "")
	assert.Equal(t, http.StatusBadRequest, w.Code, "4th attempt without token must be rejected")
	assert.Contains(t, w.Body.String(), "CAPTCHA_REQUIRED")
}

// TestSignupCaptchaGuard_ValidTokenPasses: once challenged, a valid token lets
// the request through and reaches the verifier.
func TestSignupCaptchaGuard_ValidTokenPasses(t *testing.T) {
	v := &stubVerifier{ok: true}
	r := newCaptchaEngine(SignupCaptchaGuard(v, CaptchaConfig{FreeAttempts: 1}))

	// Burn the single free attempt.
	assert.Equal(t, http.StatusCreated, doSignup(r, "9.9.9.9", "").Code)

	// 2nd attempt with a good token should pass and hit the verifier.
	w := doSignup(r, "9.9.9.9", "good-token")
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, 1, v.calls)
	assert.Equal(t, "good-token", v.gotToken)
}

// TestSignupCaptchaGuard_InvalidTokenRejected: a token the verifier rejects
// yields 400 CAPTCHA_REQUIRED.
func TestSignupCaptchaGuard_InvalidTokenRejected(t *testing.T) {
	v := &stubVerifier{ok: false}
	r := newCaptchaEngine(SignupCaptchaGuard(v, CaptchaConfig{FreeAttempts: 1}))

	// Burn the free attempt so the next one is challenged.
	assert.Equal(t, http.StatusCreated, doSignup(r, "5.6.7.8", "").Code)

	w := doSignup(r, "5.6.7.8", "bad-token")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "CAPTCHA_REQUIRED")
	assert.Equal(t, 1, v.calls)
}

// TestSignupCaptchaGuard_VerifierErrorFailsClosed: a transport error from the
// verifier blocks the request with 503 CAPTCHA_UNAVAILABLE rather than letting
// it through.
func TestSignupCaptchaGuard_VerifierErrorFailsClosed(t *testing.T) {
	v := &stubVerifier{err: errors.New("network down")}
	r := newCaptchaEngine(SignupCaptchaGuard(v, CaptchaConfig{FreeAttempts: 1}))

	// Burn the free attempt so the next one is challenged.
	assert.Equal(t, http.StatusCreated, doSignup(r, "5.6.7.9", "").Code)

	w := doSignup(r, "5.6.7.9", "any-token")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "CAPTCHA_UNAVAILABLE")
}

// TestSignupCaptchaGuard_IsolatesByIP: one abusive IP exhausting its quota does
// not force a challenge on a different IP.
func TestSignupCaptchaGuard_IsolatesByIP(t *testing.T) {
	v := &stubVerifier{ok: true}
	r := newCaptchaEngine(SignupCaptchaGuard(v, CaptchaConfig{FreeAttempts: 1}))

	// IP A burns its free attempt and gets challenged on the 2nd.
	assert.Equal(t, http.StatusCreated, doSignup(r, "10.0.0.1", "").Code)
	assert.Equal(t, http.StatusBadRequest, doSignup(r, "10.0.0.1", "").Code)

	// IP B is unaffected — its first attempt is still free.
	assert.Equal(t, http.StatusCreated, doSignup(r, "10.0.0.2", "").Code)
}

// TestSignupCaptchaGuard_NilVerifierDisables: a nil verifier (unconfigured
// secret) turns the guard into a pass-through.
func TestSignupCaptchaGuard_NilVerifierDisables(t *testing.T) {
	r := newCaptchaEngine(SignupCaptchaGuard(nil, CaptchaConfig{FreeAttempts: 1}))

	for i := 0; i < 5; i++ {
		assert.Equal(t, http.StatusCreated, doSignup(r, "3.3.3.3", "").Code)
	}
}
