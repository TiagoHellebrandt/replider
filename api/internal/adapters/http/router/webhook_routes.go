package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

// setupWebhookRoutes wires the tenant-managed outbound webhook
// endpoints. Admin-only because exposing event streams is a security-
// sensitive action — a compromised webhook URL would leak inbound
// customer messages to a third party.
//
// Route prefix is `/account-webhooks` (NOT `/webhooks`) because gin's
// router tree treats `/webhooks/:channel_token` (the public inbound
// path that Telegram/Webchat post to) and `/webhooks/:id` as the same
// pattern with conflicting param names — startup panic. Different
// prefix is the cleanest separation between admin CRUD and public
// inbound delivery.
func setupWebhookRoutes(v1 *gin.RouterGroup, h *handlers.AccountWebhookHandler, authMw gin.HandlerFunc) {
	if h == nil {
		return
	}
	g := v1.Group("/account-webhooks", authMw, middleware.RequireRole("admin"))
	{
		// The catalogue (J2). Registered before "/:id" so "events" is a route
		// and not a webhook id.
		g.GET("/events", h.HandleListEvents)
		g.GET("", h.HandleList)
		g.POST("", h.HandleCreate)
		g.PUT("/:id", h.HandleUpdate)
		g.DELETE("/:id", h.HandleDelete)
		g.POST("/:id/test", h.HandleTest)
	}
}
