package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

func setupCannedResponseRoutes(v1 *gin.RouterGroup, h *handlers.CannedResponseHandler, authMw gin.HandlerFunc) {
	group := v1.Group("/canned-responses")
	group.Use(authMw, middleware.RequireRole("admin", "attendant"))
	{
		group.POST("", h.HandleCreate)
		group.GET("", h.HandleList)
		group.PUT("/:id", h.HandleUpdate)
		group.DELETE("/:id", h.HandleDelete)
	}
}
