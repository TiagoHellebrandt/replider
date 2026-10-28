package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

// aiRL caps the AI/data-spending routes per account (US-F012): persona
// generation calls the LLM, prospecting calls Google Places and mints leads,
// and the webhook test dials a customer-chosen endpoint.
func setupFunnelRoutes(v1 *gin.RouterGroup, h *handlers.FunnelHandler, authPaidMw gin.HandlerFunc, aiRL gin.HandlerFunc) {
	// The LIST is not admin work: the whole Clientes tab starts by reading it
	// (the board needs a funnel id), and a `vendedor` or `gestor` satisfies
	// neither `admin` nor the legacy pair — so the tab answered 403 for exactly
	// the roles that live in it. Same actor set as the lead reads in
	// lead_routes.go; `HandleList` masks the webhook secret for non-admins.
	// Its own group because gin runs the GROUP's middleware first: a
	// route-level RequireRole inside the admin group could only narrow it.
	reads := v1.Group("/funnels", authPaidMw, middleware.RequireRole("admin", "attendant", "vendedor"))
	{
		reads.GET("", h.HandleList)
	}
	g := v1.Group("/funnels", authPaidMw, middleware.RequireRole("admin"))
	{
		g.POST("/generate-persona", aiRL, h.HandleGeneratePersona)
		g.POST("", h.HandleCreate)
		g.GET("/:id", h.HandleGet)
		g.PUT("/:id", h.HandleUpdate)
		g.DELETE("/:id", h.HandleDelete)
		g.POST("/:id/webhook/test", aiRL, h.HandleTestWebhook)
		// US-022: prospect businesses via Google Places and create leads.
		g.POST("/:id/prospecting", aiRL, h.HandleProspecting)
		// Preview costs a directory call, so it shares the same limiter as
		// the search it stands in for. The import writes rows the preview
		// already paid to find, so it does not.
		g.POST("/:id/prospecting/preview", aiRL, h.HandlePreviewProspecting)
		g.POST("/:id/prospecting/import", h.HandleImportProspecting)
	}
}
