package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// MetricsAuth gates /metrics behind a static bearer token (METRICS_TOKEN).
// The scrape lists route names, tenant counts and error rates — nothing a
// stranger should read. nginx already restricts the path to the backend
// network; this is the second lock for the day a proxy rule is lost. An
// empty token disables the check (documented: local dev, or a deployment
// that relies on the network restriction alone).
func MetricsAuth(token string) gin.HandlerFunc {
	if token == "" {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		got := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer"))
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "metrics token required"})
			return
		}
		c.Next()
	}
}
