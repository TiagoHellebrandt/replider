package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
)

func setupROILeadRoutes(v1 *gin.RouterGroup, h *handlers.ROIHandler, rl func(string) gin.HandlerFunc) {
	roi := v1.Group("/roi-leads")
	{
		// Public lead capture for the ROI-calculator lead magnet (US-163).
		// Same abuse posture as the newsletter/sales endpoints: per-IP
		// fixed-window limiter on top of the use case's honeypot + per-email
		// 24h guard.
		roi.POST("", rl("5-M"), h.HandleSubmit)
	}
}
