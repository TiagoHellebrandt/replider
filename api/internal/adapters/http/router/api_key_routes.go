package router

import (
	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
)

// Machine API keys and the public API surface (§6.19, J1).

// setupAPIKeyRoutes mounts key management. Admin-only under the USER session: a
// machine key is a credential to the whole account, so minting one is not
// something an attendant does.
func setupAPIKeyRoutes(v1 *gin.RouterGroup, h *handlers.APIKeyHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := v1.Group("/api-keys", authMw, middleware.RequireRole("admin"))
	{
		// Before "/:id", so "scopes" is a route and not a key id.
		g.GET("/scopes", h.HandleListScopes)
		g.GET("", h.HandleList)
		g.POST("", h.HandleCreate)
		g.DELETE("/:id", h.HandleRevoke)
	}
}

// setupPublicAPIRoutes mounts the machine-authenticated surface.
//
// A SEPARATE tree under /public rather than key-auth bolted onto the dashboard
// routes. Two reasons, both about blast radius: a dashboard route gains a
// behaviour and inherits it for machines silently, and a route written for a
// person assumes a `userID` on the context that a key does not have. Everything
// here is opted into explicitly, one route at a time, with the scope it needs.
func setupPublicAPIRoutes(
	v1 *gin.RouterGroup,
	exportHandler *handlers.ExportHandler,
	analyticsHandler *handlers.AnalyticsHandler,
	webhookHandler *handlers.AccountWebhookHandler,
	keyAuth gin.HandlerFunc,
	keyRateLimit gin.HandlerFunc,
) {
	if keyAuth == nil {
		return
	}
	g := v1.Group("/public", keyAuth, keyRateLimit)
	{
		if exportHandler != nil {
			// The export already records who ran it; under a key the audit row
			// carries the key's name instead of a person's, which is exactly
			// what "which integration took the customer list" needs.
			g.GET("/export/:entity",
				middleware.RequireAPIScope(domain.ScopeExportRead),
				exportHandler.HandleUniversalExport)
		}
		if analyticsHandler != nil {
			g.GET("/analytics/sdr",
				middleware.RequireAPIScope(domain.ScopeAnalyticsRead),
				analyticsHandler.HandleGetSDRAnalytics)
			g.GET("/analytics/sdr/leads",
				middleware.RequireAPIScope(domain.ScopeAnalyticsRead),
				analyticsHandler.HandleDrillSDRAnalytics)
		}
		if webhookHandler != nil {
			g.GET("/webhooks/events", webhookHandler.HandleListEvents)
			g.GET("/webhooks",
				middleware.RequireAPIScope(domain.ScopeWebhooksWrite),
				webhookHandler.HandleList)
			g.POST("/webhooks",
				middleware.RequireAPIScope(domain.ScopeWebhooksWrite),
				webhookHandler.HandleCreate)
			g.PUT("/webhooks/:id",
				middleware.RequireAPIScope(domain.ScopeWebhooksWrite),
				webhookHandler.HandleUpdate)
			g.DELETE("/webhooks/:id",
				middleware.RequireAPIScope(domain.ScopeWebhooksWrite),
				webhookHandler.HandleDelete)
		}
	}
}
