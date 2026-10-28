package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

func setupCampaignRoutes(v1 *gin.RouterGroup, h *handlers.CampaignHandler, authMw gin.HandlerFunc) {
	g := v1.Group("/campaigns", authMw, middleware.RequireRole("admin"))
	{
		g.GET("", h.HandleListCampaigns)
		g.POST("", h.HandleCreateCampaign)
		g.GET("/:id", h.HandleGetCampaign)
		g.DELETE("/:id", h.HandleDeleteCampaign)
		g.GET("/:id/recipients", h.HandleListRecipients)
		g.POST("/:id/cancel", h.HandleCancelCampaign)
	}
}
