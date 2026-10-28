package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// ImpersonationAudit pulls the impersonating actor from the gin context
// (auth middleware sets it from the JWT claim), promotes it onto
// `c.Request.Context()` via domain.WithImpersonator so use cases can
// pull the value via domain.GetImpersonator on audit writes, and
// slog-WARNs every state-changing request so ops have a forensic
// trail even before the audit-table stamping rolls out across every
// use case.
//
// MUST be wired AFTER AuthMiddleware so the actor is already known.
func ImpersonationAudit() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := c.GetString("impersonatingActor")
		if actor == "" {
			return
		}
		c.Request = c.Request.WithContext(
			domain.WithImpersonator(c.Request.Context(), actor),
		)

		// Forensic trail: log every write as the operator. Reads stay
		// quiet (impersonation viewing data is expected).
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			return
		}
		slog.Warn("impersonation: write request",
			"actor", actor,
			"target_account_id", c.GetString("accountID"),
			"target_user_id", c.GetString("userID"),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"remote_addr", c.ClientIP(),
		)
	}
}
