package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
)

func setupNewsletterRoutes(v1 *gin.RouterGroup, h *handlers.NewsletterHandler, rl func(string) gin.HandlerFunc) {
	newsletter := v1.Group("/newsletter")
	{
		newsletter.POST("/subscribe", rl("5-M"), h.HandleSubscribe)
	}
}
