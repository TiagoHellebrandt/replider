package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

// setupPushRoutes mounts the push-subscription CRUD. Open to admins and
// attendants since both receive new-message pings from the inbound pipeline.
func setupPushRoutes(v1 *gin.RouterGroup, h *handlers.PushHandler, authMw gin.HandlerFunc) {
	group := v1.Group("/push")
	group.Use(authMw, middleware.RequireRole("admin", "attendant"))
	{
		group.GET("/vapid-public-key", h.HandleVAPIDPublicKey)
		group.GET("/subscriptions", h.HandleList)
		group.POST("/subscriptions", h.HandleSubscribe)
		group.DELETE("/subscriptions", h.HandleUnsubscribe)
		group.POST("/test", h.HandleSendTest)
	}
}
