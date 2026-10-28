package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

func setupWSRoutes(v1 *gin.RouterGroup, wsHandler *handlers.WSHandler, authMw gin.HandlerFunc, rateLimit gin.HandlerFunc) {
	// WebSocket supports three auth methods (see OptionalAuthMiddleware):
	// 1. Authorization: Bearer header on the upgrade request (preferred for native clients)
	// 2. Query param ?token= (DEPRECATED — leaks to logs/URLs; logs a warning)
	// 3. First-frame auth — upgrade without auth, token sent as first message (browsers)
	// OptionalAuthMiddleware allows unauthenticated upgrade; WSHandler.authenticateViaFrame handles case 3.
	v1.GET("/ws/chat", rateLimit, middleware.OptionalAuthMiddleware(authMw), wsHandler.HandleConnections)
}
