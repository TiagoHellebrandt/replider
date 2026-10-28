package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

func setupAgentRoutes(v1 *gin.RouterGroup, agentHandler *handlers.AgentHandler, templatesHandler *handlers.AgentTemplatesHandler, calendarHandler *handlers.CalendarHandler, authMw gin.HandlerFunc, rl func(string) gin.HandlerFunc) {
	// Mission 7: agent-templates catalogue. Auth-gated (not public)
	// so the prompts don't become crawler bait; kept admin-only to
	// match /agents access.
	templates := v1.Group("/agent-templates")
	templates.Use(authMw, middleware.RequireRole("admin"))
	{
		templates.GET("", templatesHandler.HandleList)
		templates.GET("/:id", templatesHandler.HandleGet)
	}

	agents := v1.Group("/agents")
	agents.Use(authMw, middleware.RequireRole("admin"))
	{
		agents.POST("", agentHandler.HandleCreateAgent)
		agents.GET("", agentHandler.HandleListAgents)
		// Preview burns provider tokens on each call, so we stack an IP limit
		// (anti-brute) AND a per-account limit (anti-abuse by the logged-in
		// user's own integrations). Either hit returns 429.
		agents.POST("/preview", rl("10-M"), middleware.RateLimiterByAccount("60-M"), agentHandler.HandlePreviewChat)
		agents.POST("/:id/learn-personality", middleware.RateLimiterByAccount("10-H"), agentHandler.HandleLearnPersonality)
		agents.PUT("/:id", agentHandler.HandleUpdateAgent)
		agents.PATCH("/:id/toggle", agentHandler.HandlePauseAgent)
		agents.DELETE("/:id", agentHandler.HandleDeleteAgent)

		// Calendar routes nested under agents
		agents.POST("/:id/calendars", calendarHandler.HandleCreateCalendar)
		agents.GET("/:id/calendars", calendarHandler.HandleListCalendars)
		agents.PUT("/:id/calendars/:calendarId", calendarHandler.HandleUpdateCalendar)
		agents.DELETE("/:id/calendars/:calendarId", calendarHandler.HandleDeleteCalendar)
		agents.POST("/:id/calendars/:calendarId/test-connection", calendarHandler.HandleTestCalendarConnection)
	}
}
