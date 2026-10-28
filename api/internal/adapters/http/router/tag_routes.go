package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

func setupTagRoutes(v1 *gin.RouterGroup, h *handlers.TagHandler, authMw gin.HandlerFunc) {
	// Tag CRUD (admin only)
	tags := v1.Group("/tags")
	tags.Use(authMw, middleware.RequireRole("admin"))
	{
		tags.POST("", h.HandleCreate)
		tags.GET("", h.HandleList)
		tags.PUT("/:id", h.HandleUpdate)
		tags.DELETE("/:id", h.HandleDelete)
	}

	// Conversation tag management (admin + attendant)
	convTags := v1.Group("/conversations")
	convTags.Use(authMw, middleware.RequireRole("admin", "attendant"))
	{
		convTags.GET("/:id/tags", h.HandleListConversationTags)
		convTags.POST("/:id/tags", h.HandleAddToConversation)
		convTags.DELETE("/:id/tags/:tagId", h.HandleRemoveFromConversation)
	}
}
