package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// OAuthMetadataHandler serves the two discovery documents MCP clients
// use to find the authorization server: RFC 9728 (protected resource)
// and RFC 8414 (authorization server metadata).
type OAuthMetadataHandler struct {
	issuer string // public API origin, e.g. https://api.replider.com
}

func NewOAuthMetadataHandler(issuer string) *OAuthMetadataHandler {
	return &OAuthMetadataHandler{issuer: issuer}
}

// HandleProtectedResourceMetadata implements RFC 9728. MCP clients hit
// this after a 401 whose WWW-Authenticate header points here.
func (h *OAuthMetadataHandler) HandleProtectedResourceMetadata(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"resource":                 h.issuer + "/mcp",
		"authorization_servers":    []string{h.issuer},
		"bearer_methods_supported": []string{"header"},
		"scopes_supported":         []string{"mcp"},
	})
}

// HandleAuthorizationServerMetadata implements RFC 8414.
func (h *OAuthMetadataHandler) HandleAuthorizationServerMetadata(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"issuer":                                h.issuer,
		"authorization_endpoint":                h.issuer + "/oauth/authorize",
		"token_endpoint":                        h.issuer + "/oauth/token",
		"registration_endpoint":                 h.issuer + "/oauth/register",
		"revocation_endpoint":                   h.issuer + "/oauth/revoke",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none", "client_secret_basic", "client_secret_post"},
		"scopes_supported":                      []string{"mcp"},
	})
}
