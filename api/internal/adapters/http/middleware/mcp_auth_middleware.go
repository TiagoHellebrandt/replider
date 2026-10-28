package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// MCPAuthMiddleware authenticates the /mcp surface with unified bearer
// credentials: OAuth access tokens (Claude.ai connector) and personal
// access tokens (Claude Code / API), both resolved by one hash lookup.
// It sets the SAME gin context keys as AuthMiddleware so every
// downstream usecase and RequireRole works unchanged.
//
// Per the MCP spec, 401 responses carry a WWW-Authenticate header
// pointing at the protected-resource metadata so OAuth-capable clients
// can discover the authorization server.
func MCPAuthMiddleware(resolver domain.APITokenResolver, resourceMetadataURL string) gin.HandlerFunc {
	challengeBase := fmt.Sprintf(`Bearer realm="replider-mcp", resource_metadata=%q`, resourceMetadataURL)

	return func(c *gin.Context) {
		unauthorized := func(withError bool, desc string) {
			challenge := challengeBase
			if withError {
				challenge += `, error="invalid_token"`
			}
			c.Header("WWW-Authenticate", challenge)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "UNAUTHORIZED",
				"message": desc,
			})
		}

		// Bearer header only — no query-param fallback on this surface
		// (MCP clients always support headers; query tokens leak in logs).
		var raw string
		if authHeader := c.GetHeader("Authorization"); authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				raw = strings.TrimSpace(parts[1])
			}
		}
		if raw == "" {
			unauthorized(false, "authorization required")
			return
		}

		resolved, err := resolver.Resolve(c.Request.Context(), domain.HashToken(raw))
		if err != nil {
			// Fail CLOSED: a DB error must never grant access.
			slog.Error("mcp auth: token resolve failed", "error", err)
			unauthorized(true, "invalid or expired token")
			return
		}
		if resolved == nil || !resolved.UserActive {
			unauthorized(true, "invalid or expired token")
			return
		}
		// Tenant-boundary belt-and-suspenders, mirroring AuthMiddleware.
		if resolved.UserID == "" || resolved.AccountID == "" {
			unauthorized(true, "invalid token claims")
			return
		}

		c.Set("userID", resolved.UserID)
		c.Set("userName", resolved.UserName)
		c.Set("accountID", resolved.AccountID)
		c.Set("role", resolved.Role)
		c.Set("authVia", "mcp_token")

		// Best-effort last-used bookkeeping, detached from the request
		// lifecycle (G118: WithoutCancel, not Background).
		go resolver.TouchLastUsed(context.WithoutCancel(c.Request.Context()), resolved.TokenID)

		// No c.Next() — same gin auto-advance convention as AuthMiddleware.
	}
}
