package middleware

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// ctxTokenIssuedAt is where AuthMiddleware leaves the token's iat for the
// guards that run after it.
const ctxTokenIssuedAt = "tokenIssuedAt"

// sessionGuardCacheTTL bounds how stale the invalidation stamp may be. A
// removed member keeps a live token for at most this long; in exchange the
// guard costs one query per user per TTL rather than one per request.
const sessionGuardCacheTTL = 30 * time.Second

type sessionStamp struct {
	at      *time.Time
	fetched time.Time
}

// SessionInvalidationGuard refuses a token issued before the user's
// users.session_invalidated_at (stamped on member removal). Runs after
// AuthMiddleware, which stores userID and the token's iat on the context.
// A nil reader (test doubles, or a repository without the method) is a
// pass-through, so the guard can never lock everyone out by accident.
func SessionInvalidationGuard(reader domain.SessionInvalidationReader) gin.HandlerFunc {
	if reader == nil {
		return func(c *gin.Context) { c.Next() }
	}
	var mu sync.Mutex
	cache := map[string]sessionStamp{}

	lookup := func(c *gin.Context, userID string) (*time.Time, bool) {
		now := time.Now()
		mu.Lock()
		if s, ok := cache[userID]; ok && now.Sub(s.fetched) < sessionGuardCacheTTL {
			mu.Unlock()
			return s.at, true
		}
		mu.Unlock()

		at, err := reader.SessionInvalidatedAt(c.Request.Context(), userID)
		if err != nil {
			// Fail open on a read error: the blacklist and expiry still
			// apply, and a database blip must not log every user out.
			slog.WarnContext(c.Request.Context(), "session guard: stamp lookup failed",
				"user_id", userID, "error", err)
			return nil, false
		}
		mu.Lock()
		if len(cache) > 10000 {
			// Crude bound; the map is per-user and users are few.
			cache = map[string]sessionStamp{}
		}
		cache[userID] = sessionStamp{at: at, fetched: now}
		mu.Unlock()
		return at, true
	}

	return func(c *gin.Context) {
		userID := c.GetString("userID")
		issuedAt, _ := c.Get(ctxTokenIssuedAt)
		iat, _ := issuedAt.(time.Time)
		if userID == "" || iat.IsZero() {
			c.Next()
			return
		}
		at, ok := lookup(c, userID)
		if ok && domain.SessionInvalidatedBefore(iat, at) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":  "AUTH_SESSION_REVOKED",
				"error": "session has been revoked",
			})
			return
		}
		c.Next()
	}
}
