package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// OptionalAuthMiddleware wraps an auth middleware to make it non-blocking.
// It supports three WebSocket auth flows, in order of preference:
//  1. Authorization: Bearer <token> header on the upgrade request (preferred —
//     native clients can set headers; the token never touches access logs / URLs).
//  2. ?token=<token> query param (DEPRECATED — leaks into proxy/access logs and
//     browser history; kept working for backwards compatibility, logs a warning).
//  3. No token at all → continue without user context so WSHandler can perform
//     first-frame auth (the token arrives in the first WebSocket message).
func OptionalAuthMiddleware(authMw gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		hasHeader := c.GetHeader("Authorization") != ""
		hasQueryToken := c.Query("token") != ""

		if hasHeader {
			authMw(c)
			return
		}
		if hasQueryToken {
			// US-193: the query-param token still authenticates, but it is
			// deprecated because it can leak into access logs and URL history.
			// Clients should send `Authorization: Bearer <token>` on the
			// handshake (native) or use first-frame auth (browser).
			slog.WarnContext(c.Request.Context(),
				"deprecated WebSocket auth via ?token= query param; use Authorization header or first-frame auth",
				"path", c.FullPath(),
			)
			authMw(c)
			return
		}
		c.Next()
	}
}

// AuthMiddleware creates a gin middleware for JWT authentication.
// If blacklist is non-nil, revoked tokens are rejected even if they
// haven't expired yet (real logout support).
func AuthMiddleware(jwtService domain.JWTService, blacklist ...domain.TokenBlacklist) gin.HandlerFunc {
	var bl domain.TokenBlacklist
	if len(blacklist) > 0 {
		bl = blacklist[0]
	}

	return func(c *gin.Context) {
		var tokenString string

		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
			}
		}

		// Fallback: accept token from query param ONLY on the WebSocket
		// handshake, where browsers cannot set headers on the upgrade. On
		// any other route a ?token= would land in proxy access logs,
		// browser history and Referer headers for no reason.
		if tokenString == "" && queryTokenAllowed(c) {
			tokenString = c.Query("token")
		}

		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization required"})
			return
		}
		claims, err := jwtService.ValidateToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		// Reject claims with empty tenant-scope fields. A forged/malformed
		// token that decodes successfully but carries an empty AccountID
		// or UserID would let downstream queries silently fall through to
		// "WHERE account_id = ''" — which matches nothing on legitimate
		// data but is a denial-of-isolation risk if any future code path
		// treats empty-string as a wildcard. Belt-and-suspenders for the
		// tenant boundary (US-004).
		if claims.AccountID == "" || claims.UserID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		// Check token blacklist (revoked via logout)
		if bl != nil && bl.IsRevoked(c.Request.Context(), tokenString) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token has been revoked"})
			return
		}

		// Store user details in the context
		if claims.IssuedAt != nil {
			// Read by SessionInvalidationGuard: a token minted before the
			// user's sessions were invalidated is refused.
			c.Set(ctxTokenIssuedAt, claims.IssuedAt.Time)
		}
		c.Set("userID", claims.UserID)
		c.Set("userName", claims.UserName)
		c.Set("accountID", claims.AccountID)
		c.Set("role", claims.Role)
		c.Set("token", tokenString) // Store for logout handler
		// Impersonation context — non-empty means a super-admin minted
		// this token via the owner panel. The /me handler echoes this
		// so the dashboard renders a banner; audit code stamps it on
		// every write.
		if claims.ImpersonatingActor != "" {
			c.Set("impersonatingActor", claims.ImpersonatingActor)
		}

		// Intentionally NOT calling c.Next(): gin's outer dispatch
		// loop auto-advances to the next handler when this returns
		// without aborting. The explicit c.Next() call would also
		// run the WHOLE remaining chain inline, which broke the
		// authPaidMw composite (subscription check ran AFTER the
		// handler had already written 200 + body, producing a
		// concatenated 200 + 402 response that crashed the Flutter
		// JSON parser with "Unexpected non-whitespace character
		// after JSON at position 624").
	}
}

// queryTokenAllowed reports whether this route may authenticate via the
// ?token= query parameter. Only the WebSocket handshake qualifies: it is
// the one place a browser cannot send an Authorization header.
func queryTokenAllowed(c *gin.Context) bool {
	return strings.Contains(c.FullPath(), "/ws/")
}
