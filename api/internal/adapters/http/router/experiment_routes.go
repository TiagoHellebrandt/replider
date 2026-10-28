package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

// setupExperimentRoutes wires the A/B testing endpoints. Plan-tier
// gating (Pro+) is enforced inside the use case via Plan.AllowsExperiments,
// not here, so a Solo/Starter admin gets a clean PLAN_FEATURE_LOCKED
// (403) instead of being silently blocked at the route layer.
//
// suggestRL applies a per-account rate limit to the auto-suggester
// path (5/day in production). The limiter is built by the caller from
// the same factory the rest of the app uses, so test environments get
// the no-op stub automatically.
func setupExperimentRoutes(
	v1 *gin.RouterGroup,
	h *handlers.ExperimentHandler,
	authMw gin.HandlerFunc,
	suggestRL gin.HandlerFunc,
) {
	agents := v1.Group("/agents/:id/experiments")
	agents.Use(authMw, middleware.RequireRole("admin"))
	{
		agents.POST("", h.HandleCreate)
		agents.GET("", h.HandleListByAgent)
	}
	exp := v1.Group("/experiments/:id")
	exp.Use(authMw, middleware.RequireRole("admin"))
	{
		exp.GET("", h.HandleGet)
		exp.POST("/start", h.HandleStart)
		exp.POST("/stop", h.HandleStop)
		exp.GET("/results", h.HandleResults)
		// Suggester: rate-limited and additionally gated on
		// Plan.AllowsAutoSuggest inside the use case (Business+).
		exp.POST("/suggest-improvement", suggestRL, h.HandleSuggest)
	}
}
