package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
)

// Machine API key authentication (§6.19, J1).
//
// A separate middleware from the user JWT rather than a branch inside it: the two
// answer different questions ("which person?" vs "which integration?"), and a
// single middleware that resolved either would eventually let a route written for
// a person be reached by a key that has no user at all.

// APIKeyResolver is the narrow port the middleware depends on.
type APIKeyResolver interface {
	ResolveKey(ctx context.Context, tokenHash string) (*domain.ResolvedAPIKey, error)
	TouchLastUsed(ctx context.Context, keyID string)
}

// Gin context keys the downstream handlers read.
const (
	apiKeyScopesKey = "apiKeyScopes"
	apiKeyNameKey   = "apiKeyName"
	apiKeyIDKey     = "apiKeyID"
)

// APIKeyAuth authenticates a machine key and puts the account on the context.
//
// It populates `accountID` with the same key the JWT path uses, so every
// downstream handler's account scoping works untouched. It deliberately does NOT
// populate `userID` or `role`: a key is not a person, and inventing a user id
// would put a name on audit rows that no human touched.
func APIKeyAuth(resolver APIKeyResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimSpace(c.GetHeader("Authorization"))
		token := strings.TrimSpace(strings.TrimPrefix(raw, "Bearer"))
		if token == "" || !strings.HasPrefix(token, domain.TokenPrefixAPIKey) {
			// The prefix check makes a user JWT pasted into a machine
			// integration fail as "wrong kind of credential" rather than as a
			// mysterious 401.
			c.AbortWithStatusJSON(http.StatusUnauthorized, domain.AppError{
				Code:    "API_KEY_REQUIRED",
				Message: "envie uma chave de máquina (rpk_...) no header Authorization",
			})
			return
		}

		resolved, err := resolver.ResolveKey(c.Request.Context(), domain.HashToken(token))
		if err != nil {
			// A database problem is not a bad key: 503 so a monitoring system
			// can tell an outage from an integration with the wrong secret.
			slog.ErrorContext(c.Request.Context(), "api key: resolve failed", "error", err)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, domain.AppError{
				Code: "API_KEY_UNAVAILABLE", Message: "could not validar a chave"})
			return
		}
		if resolved == nil {
			// One answer for revoked, expired, wrong-type and never-existed:
			// distinguishing them would make this endpoint an oracle.
			c.AbortWithStatusJSON(http.StatusUnauthorized, domain.AppError{
				Code: "API_KEY_INVALID", Message: "chave inválida ou revogada"})
			return
		}

		c.Set("accountID", resolved.AccountID)
		c.Set(apiKeyIDKey, resolved.KeyID)
		c.Set(apiKeyNameKey, resolved.Name)
		c.Set(apiKeyScopesKey, resolved.Scopes)

		// Best-effort and throttled in SQL: an integration must not fail
		// because the "last used" write did.
		resolver.TouchLastUsed(c.Request.Context(), resolved.KeyID)

		c.Next()
	}
}

// RequireAPIScope refuses a key that lacks the scope this route needs.
//
// Applied per route rather than per group, because the scope is a property of the
// endpoint: "leads:read" on a listing and "leads:write" on the create, in the same
// group.
func RequireAPIScope(required domain.APIScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopes, _ := c.Get(apiKeyScopesKey)
		granted, _ := scopes.([]domain.APIScope)
		if !domain.HasAPIScope(granted, required) {
			// Names the missing scope: "403" alone sends somebody back to the
			// documentation, while the name is a one-line fix.
			c.AbortWithStatusJSON(http.StatusForbidden, domain.ErrAPIScopeMissing(required))
			return
		}
		c.Next()
	}
}

// APIKeyName is the integration's name, for audit rows that have no person.
func APIKeyName(c *gin.Context) string {
	name, _ := c.Get(apiKeyNameKey)
	s, _ := name.(string)
	return s
}
