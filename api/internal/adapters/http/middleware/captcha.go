package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// US-054: escalating CAPTCHA challenge on the public signup endpoint.
//
// Rationale: US-053 already caps signups at 5/hour/IP, but a hard cap either
// blocks legitimate retries or lets 5 spam accounts through every hour. A
// captcha challenge is a softer, smarter gate — the first few attempts from an
// IP are free (a real human almost never needs more), and only once an IP looks
// abusive do we demand proof-of-human. This keeps the happy path frictionless
// while making automated account-creation expensive.

const (
	// DefaultCaptchaFreeAttempts is how many signups an IP may make within
	// the window before a captcha token becomes mandatory. The 4th attempt
	// in the window is the first that must carry a token — i.e. "triggered
	// after 3 attempts/hour from same IP".
	DefaultCaptchaFreeAttempts = 3

	// DefaultCaptchaWindow is the rolling window over which attempts are
	// counted before the count resets.
	DefaultCaptchaWindow = time.Hour

	// captchaTokenHeader is where the client sends the Turnstile/hCaptcha
	// response token. A header keeps the JSON request body untouched so the
	// downstream handler binds it normally.
	captchaTokenHeader = "X-Captcha-Token" //nolint:gosec // G101: HTTP header name, not a credential

	// captchaTokenQuery is an alternate location for the token, handy for
	// clients that can't set custom headers.
	captchaTokenQuery = "captcha_token" //nolint:gosec // G101: query param name, not a credential
)

// CaptchaVerifier validates a CAPTCHA response token. Returns (true, nil) when
// the challenge passed, (false, nil) when it provably failed (bad/empty token),
// and (false, err) when the check itself couldn't be completed (network/parse).
type CaptchaVerifier interface {
	Verify(ctx context.Context, token, remoteIP string) (bool, error)
}

// captchaCounter tracks attempts for one IP within the active window.
type captchaCounter struct {
	count       int
	windowStart time.Time
}

// SignupCaptchaGuard returns a Gin middleware that requires a valid captcha
// token once an IP exceeds cfg.FreeAttempts within cfg.Window. Attempts up to
// and including the free quota pass straight through; subsequent attempts must
// carry a token that verifier accepts, otherwise the request is rejected with
// 400 CAPTCHA_REQUIRED.
//
// A nil verifier disables the guard entirely (pass-through) — this is the
// local-dev / unconfigured-secret behaviour, mirroring how the rate limiters
// no-op in the test environment.
func SignupCaptchaGuard(verifier CaptchaVerifier, cfg CaptchaConfig) gin.HandlerFunc {
	if verifier == nil {
		return func(c *gin.Context) { c.Next() }
	}

	free := cfg.FreeAttempts
	if free <= 0 {
		free = DefaultCaptchaFreeAttempts
	}
	window := cfg.Window
	if window <= 0 {
		window = DefaultCaptchaWindow
	}

	var mu sync.Mutex
	counters := make(map[string]*captchaCounter)

	// Janitor: evict counters whose window has fully elapsed so the map
	// can't grow without bound under a churn of distinct IPs.
	go func() {
		ticker := time.NewTicker(window)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			mu.Lock()
			for k, ctr := range counters {
				if now.Sub(ctr.windowStart) >= window {
					delete(counters, k)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		ctr, ok := counters[ip]
		if !ok || now.Sub(ctr.windowStart) >= window {
			ctr = &captchaCounter{count: 0, windowStart: now}
			counters[ip] = ctr
		}
		priorAttempts := ctr.count
		// Count this attempt regardless of outcome so an attacker can't
		// stay under the threshold by spraying invalid tokens.
		ctr.count++
		mu.Unlock()

		if priorAttempts < free {
			// Still within the free quota — no challenge yet.
			c.Next()
			return
		}

		token := c.GetHeader(captchaTokenHeader)
		if token == "" {
			token = c.Query(captchaTokenQuery)
		}
		if token == "" {
			// Challenged but no token supplied — reject without burning a
			// verifier round-trip.
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"code":    "CAPTCHA_REQUIRED",
				"message": "captcha verification required — too many attempts from your network",
			})
			return
		}

		ok, err := verifier.Verify(c.Request.Context(), token, ip)
		if err != nil {
			// Fail closed: if we can't verify, we don't let the request
			// through, but we surface a distinct code so ops can tell a
			// provider outage from a genuine bot.
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"code":    "CAPTCHA_UNAVAILABLE",
				"message": "could not verify captcha, please try again",
			})
			return
		}
		if !ok {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"code":    "CAPTCHA_REQUIRED",
				"message": "captcha verification required — too many attempts from your network",
			})
			return
		}

		c.Next()
	}
}

// CaptchaConfig tunes SignupCaptchaGuard. Both fields are optional; zero values
// fall back to the Default* constants.
type CaptchaConfig struct {
	// FreeAttempts is the number of un-challenged attempts per IP per
	// window. <=0 → DefaultCaptchaFreeAttempts.
	FreeAttempts int
	// Window is the rolling counting window. <=0 → DefaultCaptchaWindow.
	Window time.Duration
}
