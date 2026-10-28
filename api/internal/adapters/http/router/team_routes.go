package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

func setupTeamRoutes(v1 *gin.RouterGroup, teamHandler *handlers.TeamHandler, authMw gin.HandlerFunc) {
	team := v1.Group("/team")
	team.Use(authMw, middleware.RequireRole("admin"))
	{
		team.GET("", teamHandler.HandleListMembers)
		team.POST("", teamHandler.HandleInviteMember)
		team.PATCH("/:id/role", teamHandler.HandleUpdateRole)
		team.DELETE("/:id", teamHandler.HandleRemoveMember)
		team.POST("/:id/resend-invite", teamHandler.HandleResendInvite)
		// US-016: round-robin config. Note the literal segment coexists
		// with the :id param routes above because gin matches static
		// segments first.
		team.GET("/auto-assign", teamHandler.HandleGetAutoAssign)
		team.PUT("/auto-assign", teamHandler.HandleUpdateAutoAssign)
	}
}
