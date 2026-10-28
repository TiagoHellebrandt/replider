package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// US-051: token-bucket request rate limiter keyed per (account_id, ip).
//
// Why a second limiter alongside RateLimiter/RateLimiterByAccount: those use
// ulule/limiter's fixed-window counter and key by EITHER ip OR account. This
// one uses a true token bucket (golang.org/x/time/rate) keyed by the COMPOSITE
// (account_id, ip), which is the granularity an audit asks for — a single
// account can't fan out across IPs to multiply its budget, and an
// unauthenticated client is still bucketed by IP. It also emits a
// spec-accurate Retry-After header derived from the bucket refill delay.
const (
	// DefaultRateLimitPerMinute is the sustained request budget per
	// (account_id, ip) when RateLimitConfig leaves it unset.
	DefaultRateLimitPerMinute = 100

	// rateLimitIdleTTL bounds memory: a bucket untouched for this long is
	// evicted by the janitor so the map can't grow without limit.
	rateLimitIdleTTL = 10 * time.Minute
)

// RateLimitConfig configures PerAccountIPRateLimiter. Both fields are optional;
// zero values fall back to sensible defaults so callers can pass an empty
// struct for the default 100 req/min policy. Per-plan tuning is achieved by
// constructing the middleware with a higher RequestsPerMinute for premium
// route groups.
type RateLimitConfig struct {
	// RequestsPerMinute is the sustained refill rate. <=0 → DefaultRateLimitPerMinute.
	RequestsPerMinute int
	// Burst is the bucket capacity (max instantaneous requests). <=0 → RequestsPerMinute.
	Burst int
}

type rateLimitBucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// PerAccountIPRateLimiter returns a Gin middleware enforcing a token-bucket
// rate limit keyed by the composite (account_id, client_ip). When the request
// is unauthenticated the account portion is empty, degrading gracefully to a
// per-IP limit. On exhaustion it aborts with 429 and a Retry-After header (in
// seconds) computed from the bucket's refill delay.
func PerAccountIPRateLimiter(cfg RateLimitConfig) gin.HandlerFunc {
	return keyedRateLimiter(cfg, func(c *gin.Context) string {
		return c.GetString("accountID") + "|" + c.ClientIP()
	})
}

// PerConversationRateLimiter returns a Gin middleware enforcing a token-bucket
// rate limit keyed by the conversation id path param (`:id`). It caps how fast
// messages can be pushed into a single conversation (US-052), independent of
// the per-account/per-IP budget — so one runaway integration can't spam a
// conversation thread even while staying under the global request ceiling. On
// exhaustion it aborts with 429 + Retry-After. Requests without an `:id` param
// pass through (the per-conversation cap is meaningless without a target).
func PerConversationRateLimiter(cfg RateLimitConfig) gin.HandlerFunc {
	return keyedRateLimiter(cfg, func(c *gin.Context) string {
		return c.Param("id")
	})
}

// LoginRateLimiter caps credential-stuffing attempts on the login endpoint
// (US-F012). It keys on the COMPOSITE (client_ip, submitted_email) rather
// than IP alone, which is what makes it useful in both directions:
//
//   - one IP hammering many accounts is still capped per target account;
//   - a botnet spreading one account across many IPs is capped per email.
//
// The per-IP `rl("10-M")` limiter stays in front for the raw flood case;
// this is the narrower, per-credential budget the audit asks for.
//
// Reading the email means consuming the request body, so it is restored
// for the handler. A body that isn't JSON, or carries no email, keys on IP
// alone — never pass through unlimited.
func LoginRateLimiter(cfg RateLimitConfig) gin.HandlerFunc {
	return keyedRateLimiter(cfg, func(c *gin.Context) string {
		email := loginEmailFromBody(c)
		return "login|" + c.ClientIP() + "|" + email
	})
}

// maxLoginBodyPeek bounds how much of the body we buffer to find the email.
// A login payload is a few hundred bytes; anything larger is not a login.
const maxLoginBodyPeek = 8 << 10

func loginEmailFromBody(c *gin.Context) string {
	if c.Request == nil || c.Request.Body == nil {
		return ""
	}
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxLoginBodyPeek))
	if err != nil {
		return ""
	}
	// Restore the body so the handler's ShouldBindJSON still works. Reading
	// it here is invisible to everything downstream.
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))

	var payload struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(payload.Email))
}

// keyedRateLimiter is the shared token-bucket engine: it maintains one
// rate.Limiter per key (returned by keyFn) and enforces the configured
// refill/burst. An empty key from keyFn means "no limit applies" → pass
// through. A background janitor evicts idle buckets to bound memory.
func keyedRateLimiter(cfg RateLimitConfig, keyFn func(*gin.Context) string) gin.HandlerFunc {
	rpm := cfg.RequestsPerMinute
	if rpm <= 0 {
		rpm = DefaultRateLimitPerMinute
	}
	burst := cfg.Burst
	if burst <= 0 {
		burst = rpm
	}
	limit := rate.Limit(float64(rpm) / 60.0) // tokens per second

	var mu sync.Mutex
	buckets := make(map[string]*rateLimitBucket)

	// Janitor: periodically drop idle buckets so a churn of distinct keys
	// can't leak memory. Runs for the process lifetime, same as the
	// middleware it serves.
	go func() {
		ticker := time.NewTicker(rateLimitIdleTTL)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-rateLimitIdleTTL)
			mu.Lock()
			for k, b := range buckets {
				if b.lastSeen.Before(cutoff) {
					delete(buckets, k)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		key := keyFn(c)
		if key == "" {
			c.Next()
			return
		}

		mu.Lock()
		b, ok := buckets[key]
		if !ok {
			b = &rateLimitBucket{limiter: rate.NewLimiter(limit, burst)}
			buckets[key] = b
		}
		b.lastSeen = time.Now()
		lim := b.limiter
		mu.Unlock()

		// Reserve so we can read the precise refill delay for Retry-After.
		res := lim.Reserve()
		if !res.OK() {
			// Burst is 0 — should never happen given the guard above.
			rejectRateLimited(c, 1)
			return
		}
		if delay := res.Delay(); delay > 0 {
			// No token available now: cancel the reservation (return it to
			// the bucket) and reject with the wait time as Retry-After.
			res.Cancel()
			retryAfter := int(math.Ceil(delay.Seconds()))
			if retryAfter < 1 {
				retryAfter = 1
			}
			rejectRateLimited(c, retryAfter)
			return
		}
		c.Next()
	}
}

func rejectRateLimited(c *gin.Context, retryAfterSeconds int) {
	c.Header("Retry-After", strconv.Itoa(retryAfterSeconds))
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"code":    "RATE_LIMITED",
		"message": "rate limit exceeded — slow down and retry",
	})
}

// ── Machine API keys: per-plan rate limit (§6.19, J1) ────────────────────────

// PlanRateResolver answers "how many requests per minute may this account's
// plan make?". Narrow port so the middleware does not carry an account and a
// plan repository around.
type PlanRateResolver interface {
	RequestsPerMinute(ctx context.Context, accountID string) int
}

// APIKeyRateLimiter caps machine traffic by the account's PLAN.
//
// Keyed on the account rather than on account+IP, unlike the dashboard limiter:
// an integration is one consumer no matter how many machines it runs on, and
// keying by IP would hand a customer a free multiplier for every worker they
// spin up — which is exactly the cost this cap exists to bound.
//
// A resolver that answers 0 or fails falls back to the configured default rather
// than blocking: a plan lookup problem must not take a customer's integration
// down, and the spend behind these routes is already bounded by the per-plan
// caps on everything expensive.
func APIKeyRateLimiter(cfg RateLimitConfig, resolver PlanRateResolver) gin.HandlerFunc {
	base := keyedRateLimiter(cfg, func(c *gin.Context) string {
		return "apikey|" + c.GetString("accountID")
	})
	if resolver == nil {
		return base
	}

	// One limiter per distinct rate, built lazily. Building a limiter per
	// ACCOUNT would duplicate the janitor and the bucket map for every tenant;
	// plans are few, so a handful of shared limiters covers everybody.
	var mu sync.Mutex
	byRate := map[int]gin.HandlerFunc{}

	return func(c *gin.Context) {
		accountID := c.GetString("accountID")
		if accountID == "" {
			base(c)
			return
		}
		rpm := resolver.RequestsPerMinute(c.Request.Context(), accountID)
		if rpm <= 0 {
			base(c)
			return
		}

		mu.Lock()
		limiter, ok := byRate[rpm]
		if !ok {
			scoped := cfg
			scoped.RequestsPerMinute = rpm
			limiter = keyedRateLimiter(scoped, func(c *gin.Context) string {
				return "apikey|" + c.GetString("accountID")
			})
			byRate[rpm] = limiter
		}
		mu.Unlock()

		limiter(c)
	}
}
