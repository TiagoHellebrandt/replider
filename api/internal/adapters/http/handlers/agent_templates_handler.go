package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// AgentTemplatesHandler serves the server-authoritative catalogue of
// onboarding presets (clinic / realestate / ecommerce / school).
// Stateless — all data is compiled in; no DB reads.
type AgentTemplatesHandler struct{}

func NewAgentTemplatesHandler() *AgentTemplatesHandler {
	return &AgentTemplatesHandler{}
}

// HandleList godoc
// @Summary      List agent templates (without seed knowledge)
// @Description  Returns the catalogue of onboarding presets. Seed
// @Description  knowledge is omitted on purpose — the dropdown
// @Description  doesn't need it. Use /agent-templates/{id} to get
// @Description  the full payload including chunks.
// @Tags         Agent Templates
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} domain.AgentTemplate
// @Router       /agent-templates [get]
func (h *AgentTemplatesHandler) HandleList(c *gin.Context) {
	c.JSON(http.StatusOK, domain.ListAgentTemplates())
}

// HandleGet godoc
// @Summary      Get a single agent template by ID
// @Description  Returns the full template including seed_knowledge
// @Description  (5-8 Q/A pairs). Used by clients that want to preview
// @Description  what will be indexed on agent creation.
// @Tags         Agent Templates
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Template ID (clinic|realestate|ecommerce|school)"
// @Success      200 {object} domain.AgentTemplate
// @Failure      404 {object} domain.AppError "code: TEMPLATE_NOT_FOUND"
// @Router       /agent-templates/{id} [get]
func (h *AgentTemplatesHandler) HandleGet(c *gin.Context) {
	tpl := domain.GetAgentTemplate(c.Param("id"))
	if tpl == nil {
		c.JSON(http.StatusNotFound, domain.AppError{
			Code: "TEMPLATE_NOT_FOUND", Message: "template not found",
		})
		return
	}
	c.JSON(http.StatusOK, tpl)
}
