package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

type fakeResolver struct {
	resolved *domain.ResolvedAPIToken
	err      error
	touched  chan string
}

func (f *fakeResolver) Resolve(_ context.Context, _ string) (*domain.ResolvedAPIToken, error) {
	return f.resolved, f.err
}

func (f *fakeResolver) TouchLastUsed(_ context.Context, tokenID string) {
	if f.touched != nil {
		select {
		case f.touched <- tokenID:
		default:
		}
	}
}

const testMetadataURL = "https://api.test/.well-known/oauth-protected-resource"

func mcpTestRouter(resolver domain.APITokenResolver) (*gin.Engine, *gin.Context) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var captured *gin.Context
	r.GET("/mcp", MCPAuthMiddleware(resolver, testMetadataURL), func(c *gin.Context) {
		captured = c
		c.Status(http.StatusOK)
	})
	_ = captured
	return r, captured
}

func doMCPRequest(r *gin.Engine, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/mcp", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestMCPAuth_MissingTokenReturns401WithResourceMetadata(t *testing.T) {
	r, _ := mcpTestRouter(&fakeResolver{})
	w := doMCPRequest(r, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	challenge := w.Header().Get("WWW-Authenticate")
	assert.Contains(t, challenge, `resource_metadata="`+testMetadataURL+`"`)
	// No token presented → no error attribute (per RFC 6750).
	assert.NotContains(t, challenge, "invalid_token")
}

func TestMCPAuth_InvalidTokenReturns401WithErrorAttr(t *testing.T) {
	r, _ := mcpTestRouter(&fakeResolver{resolved: nil})
	w := doMCPRequest(r, "Bearer rpl_mcp_bogus")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Header().Get("WWW-Authenticate"), "invalid_token")
}

func TestMCPAuth_DBErrorFailsClosed(t *testing.T) {
	r, _ := mcpTestRouter(&fakeResolver{err: errors.New("db down")})
	w := doMCPRequest(r, "Bearer rpl_mcp_x")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMCPAuth_InactiveUserRejected(t *testing.T) {
	r, _ := mcpTestRouter(&fakeResolver{resolved: &domain.ResolvedAPIToken{
		TokenID: "t1", UserID: "u1", AccountID: "a1", Role: "admin", UserActive: false,
	}})
	w := doMCPRequest(r, "Bearer rpl_mcp_x")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMCPAuth_ValidTokenSetsContextKeys(t *testing.T) {
	resolver := &fakeResolver{
		resolved: &domain.ResolvedAPIToken{
			TokenID: "t1", UserID: "u1", UserName: "Ana",
			AccountID: "a1", Role: "attendant", UserActive: true,
		},
		touched: make(chan string, 1),
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var gotUser, gotAccount, gotRole, gotVia string
	r.GET("/mcp", MCPAuthMiddleware(resolver, testMetadataURL), func(c *gin.Context) {
		gotUser = c.GetString("userID")
		gotAccount = c.GetString("accountID")
		gotRole = c.GetString("role")
		gotVia = c.GetString("authVia")
		c.Status(http.StatusOK)
	})
	w := doMCPRequest(r, "Bearer rpl_mcp_valid")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "u1", gotUser)
	assert.Equal(t, "a1", gotAccount)
	assert.Equal(t, "attendant", gotRole)
	assert.Equal(t, "mcp_token", gotVia)
	assert.Equal(t, "t1", <-resolver.touched)
}

func TestMCPAuth_QueryParamTokenNotAccepted(t *testing.T) {
	resolver := &fakeResolver{resolved: &domain.ResolvedAPIToken{
		TokenID: "t1", UserID: "u1", AccountID: "a1", UserActive: true,
	}}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/mcp", MCPAuthMiddleware(resolver, testMetadataURL), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest("GET", "/mcp?token=rpl_mcp_leaky", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
