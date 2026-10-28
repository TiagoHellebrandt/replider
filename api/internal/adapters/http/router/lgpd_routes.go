package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

// setupLGPDRoutes wires the admin-only LGPD titular-rights endpoints.
// These are sensitive: reading exports leaks all customer data to the
// admin, and anonymization is irreversible. Both operations are logged
// via the handler's structured logger so they show up in audit search.
func setupLGPDRoutes(v1 *gin.RouterGroup, h *handlers.LGPDHandler, authMw gin.HandlerFunc) {
	g := v1.Group("/lgpd", authMw, middleware.RequireRole("admin"))
	{
		// POST, not GET/DELETE: the data subject's phone number used to ride
		// in the query string, which writes it to every access log between
		// the browser and this process. The verb is a small price for not
		// leaking the identifier of the person exercising a privacy right.
		g.POST("/export", h.HandleExport)
		g.POST("/anonymize", h.HandleAnonymize)
	}
}

// setupAuditLogRoutes wires the read-only audit log listing. Kept separate
// so it can be disabled independently (some deployments may prefer to keep
// the audit log server-side only, viewable via DB).
func setupAuditLogRoutes(v1 *gin.RouterGroup, h *handlers.AuditLogHandler, authMw gin.HandlerFunc) {
	g := v1.Group("/audit", authMw, middleware.RequireRole("admin"))
	{
		g.GET("", h.HandleList)
		g.GET("/export.csv", h.HandleExportCSV)
	}
}
