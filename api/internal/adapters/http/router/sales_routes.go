package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
)

func setupSalesRoutes(v1 *gin.RouterGroup, h *handlers.SalesHandler, rl func(string) gin.HandlerFunc) {
	sales := v1.Group("/sales")
	{
		// Public lead capture for the Enterprise tier (US-157). Same abuse
		// posture as the newsletter endpoint: per-IP fixed-window limiter on
		// top of the use case's honeypot + per-email 24h guard.
		sales.POST("/qualify", rl("5-M"), h.HandleSubmit)
	}
}
