package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// AuditMeta stamps the request origin — client IP + User-Agent — onto the
// request context so the audit log writer records "from where" for every
// admin mutation (US-073). The repository reads it back via
// domain.GetAuditMeta; write sites don't have to thread the gin request
// down into the use-case layer.
//
// c.ClientIP() honours the trusted-proxy configuration, so behind nginx it
// resolves to the real client (X-Forwarded-For) rather than the proxy.
func AuditMeta() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := domain.WithAuditMeta(c.Request.Context(), c.ClientIP(), c.Request.UserAgent())
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
