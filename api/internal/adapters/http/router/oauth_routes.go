package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

// setupOAuthRoutes mounts the OAuth 2.1 authorization-server surface for
// the remote MCP integration. Discovery documents and /oauth/* live at
// the ROOT (RFC 8414 requires well-known paths at the origin), not /v1.
func setupOAuthRoutes(
	r *gin.Engine,
	metadataHandler *handlers.OAuthMetadataHandler,
	authorizeHandler *handlers.OAuthAuthorizeHandler,
	tokenHandler *handlers.OAuthTokenHandler,
	rl func(string) gin.HandlerFunc,
) {
	r.GET("/.well-known/oauth-protected-resource", metadataHandler.HandleProtectedResourceMetadata)
	// MCP clients derive a path-suffixed metadata URL from the resource
	// path (/mcp) — serve the same document there.
	r.GET("/.well-known/oauth-protected-resource/mcp", metadataHandler.HandleProtectedResourceMetadata)
	r.GET("/.well-known/oauth-authorization-server", metadataHandler.HandleAuthorizationServerMetadata)

	oauth := r.Group("/oauth")
	{
		// DCR is unauthenticated by spec — rate-limit hard.
		oauth.POST("/register", rl("10-H"), tokenHandler.HandleRegister)
		oauth.GET("/authorize", rl("30-M"), authorizeHandler.HandleAuthorizeGet)
		// POST carries credentials: same brute-force cap as /auth/login.
		oauth.POST("/authorize", rl("10-M"), authorizeHandler.HandleAuthorizePost)
		oauth.POST("/token", rl("30-M"), tokenHandler.HandleToken)
		oauth.POST("/revoke", rl("10-M"), tokenHandler.HandleRevoke)
	}
}

// setupMCPTokenRoutes mounts the dashboard-facing PAT + connection
// management endpoints (JWT-authenticated, not MCP-token-authenticated).
func setupMCPTokenRoutes(v1 *gin.RouterGroup, h *handlers.MCPTokenHandler, authMw gin.HandlerFunc) {
	// A PAT is how a vendedor connects their own Claude to their own
	// pipeline; the MCP tools behind it are actor-scoped, so every role
	// that may enter /mcp may mint one.
	group := v1.Group("/me", authMw, middleware.RequireRole("admin", "gestor", "vendedor", "attendant"))
	{
		group.POST("/mcp-tokens", h.HandleCreatePAT)
		group.GET("/mcp-tokens", h.HandleListPATs)
		group.DELETE("/mcp-tokens/:id", h.HandleRevokePAT)
		group.GET("/mcp-connections", h.HandleListConnections)
		group.DELETE("/mcp-connections/:clientId", h.HandleRevokeConnection)
	}
}
