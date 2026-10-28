package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequirePlatformAccount gates seller (vendedores) routes to the Replider
// platform account. Must run AFTER the auth middleware (relies on the
// "accountID" context key it stamps).
//
// Responds 404 — not 403 — outside the platform account so the routes'
// existence never leaks to regular tenants. An empty platformAccountID
// (PLATFORM_ACCOUNT_ID unset) fails closed: every request 404s.
func RequirePlatformAccount(platformAccountID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		accountID := c.GetString("accountID")
		if platformAccountID == "" || accountID == "" || accountID != platformAccountID {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "not found"})
			return
		}
		c.Next()
	}
}
