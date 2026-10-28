package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

// setupPromptRoutes wires the "Meus Prompts" library + synthesis +
// per-agent apply/rollback. All admin-only because prompts may carry
// business instructions and applying one mutates the agent's behavior.
func setupPromptRoutes(
	v1 *gin.RouterGroup,
	h *handlers.PromptHandler,
	authMw gin.HandlerFunc,
	refineRL gin.HandlerFunc,
	testScenariosRL gin.HandlerFunc,
) {
	prompts := v1.Group("/prompts")
	prompts.Use(authMw, middleware.RequireRole("admin"))
	{
		prompts.POST("", h.HandleCreate)
		prompts.GET("", h.HandleList)
		prompts.POST("/synthesize", h.HandleSynthesize)
		prompts.POST("/refine", refineRL, h.HandleRefine)
		prompts.POST("/test-scenarios", testScenariosRL, h.HandleTestScenarios)
		prompts.GET("/:id", h.HandleGet)
		prompts.PUT("/:id", h.HandleUpdate)
		prompts.DELETE("/:id", h.HandleDelete)
		prompts.POST("/:id/duplicate", h.HandleDuplicate)
	}

	agents := v1.Group("/agents/:id/prompts")
	agents.Use(authMw, middleware.RequireRole("admin"))
	{
		agents.POST("/apply", h.HandleApply)
		agents.GET("/versions", h.HandleListVersions)
		agents.POST("/versions/:vid/rollback", h.HandleRollback)
	}

	suggestions := v1.Group("/agents/:id")
	suggestions.Use(authMw, middleware.RequireRole("admin"))
	{
		suggestions.GET("/wizard-suggestions", h.HandleWizardSuggestions)
	}
}
