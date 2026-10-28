// Package mcp adapts the Model Context Protocol to Replider's core
// usecases: users connect their account to Claude (claude.ai custom
// connector, Claude Code) and manage it conversationally, with a
// proactive customer-success persona defined in instructions.go.
package mcp

import (
	"context"
	"net/http"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"

	"github.com/gin-gonic/gin"
)

// Identity is the authenticated caller, resolved by MCPAuthMiddleware.
type Identity struct {
	AccountID string
	UserID    string
	UserName  string
	Role      string
}

type identityKey struct{}

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(Identity)
	return id, ok && id.AccountID != "" && id.UserID != ""
}

// GinBridge copies the auth gin-context keys onto the request's
// context.Context so the wrapped net/http MCP handler (and every
// usecase) can read them. The reassignment of c.Request is load-bearing
// — gin context values are invisible to http.Handler code otherwise.
//
// accountRepo is optional: when given, the account's right to spend AI is
// checked here (trial expiry, inactive subscription) so the MCP mount honours
// the same rule as the paid HTTP routes — the analysis of 2026-09-01 found
// an expired trial could keep drafting through MCP. Nil keeps the old
// behaviour (tests).
func GinBridge(accountRepo domain.AccountRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := Identity{
			AccountID: c.GetString("accountID"),
			UserID:    c.GetString("userID"),
			UserName:  c.GetString("userName"),
			Role:      c.GetString("role"),
		}
		if accountRepo != nil && id.AccountID != "" {
			if err := usecases.AccountMayUseAI(c.Request.Context(), accountRepo, id.AccountID); err != nil {
				c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{"error": gin.H{
					"code": "SUBSCRIPTION_INACTIVE", "message": err.Error()}})
				return
			}
		}
		c.Request = c.Request.WithContext(WithIdentity(c.Request.Context(), id))
		c.Next()
	}
}

// Actor projects the MCP identity onto the shared authorization subject
// (US-F009). The parity rule is structural: a tool and its HTTP endpoint
// both build a domain.Actor and both hand it to the same use case, so a
// tool can never be more permissive than the screen it mirrors.
func (i Identity) Actor() domain.Actor {
	return domain.Actor{
		UserID:    i.UserID,
		AccountID: i.AccountID,
		Role:      domain.NormalizeRole(i.Role),
	}
}
