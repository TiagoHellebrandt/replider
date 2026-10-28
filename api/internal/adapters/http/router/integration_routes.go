package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

func setupIntegrationRoutes(v1 *gin.RouterGroup, h *handlers.GoogleCalendarIntegrationHandler, authMw gin.HandlerFunc, rl func(string) gin.HandlerFunc) {
	// Google's browser redirect — unauthenticated by design; the
	// HMAC-signed state carries the account identity.
	v1.GET("/oauth/google-calendar/callback", rl("30-M"), h.HandleCallback)

	integrations := v1.Group("/integrations/google-calendar")
	integrations.Use(authMw, middleware.RequireRole("admin"))
	{
		integrations.GET("", h.HandleStatus)
		integrations.POST("/connect", h.HandleConnect)
		integrations.DELETE("", h.HandleDisconnect)
		integrations.GET("/calendars", h.HandleListCalendars)
	}
}
