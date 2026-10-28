package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

type AgentHandler struct {
	previewChatUC      *usecases.PreviewChatUseCase
	createAgentUC      *usecases.CreateAgentUseCase
	listAgentsUC       *usecases.ListAgentsUseCase
	updateAgentUC      *usecases.UpdateAgentUseCase
	toggleAgentUC      *usecases.ToggleAgentStatusUseCase
	deleteAgentUC      *usecases.DeleteAgentUseCase
	learnPersonalityUC *usecases.LearnPersonalityUseCase
	// Mission 7: optional. When set, CreateAgent with a template_id
	// also seeds a "Perguntas frequentes" knowledge source using
	// the template's Q/A pairs. Nil = template prompt is applied
	// but no knowledge side-effect (acceptable for tests).
	knowledgeUC *usecases.KnowledgeUseCase
}

// SetKnowledgeUseCase wires the optional knowledge-seeding dependency.
// Kept as a setter so the existing handler_test.go that passes 7
// constructor args keeps compiling.
func (h *AgentHandler) SetKnowledgeUseCase(uc *usecases.KnowledgeUseCase) {
	h.knowledgeUC = uc
}

func NewAgentHandler(
	previewChatUC *usecases.PreviewChatUseCase,
	createAgentUC *usecases.CreateAgentUseCase,
	listAgentsUC *usecases.ListAgentsUseCase,
	updateAgentUC *usecases.UpdateAgentUseCase,
	toggleAgentUC *usecases.ToggleAgentStatusUseCase,
	deleteAgentUC *usecases.DeleteAgentUseCase,
	learnPersonalityUC *usecases.LearnPersonalityUseCase,
) *AgentHandler {
	return &AgentHandler{
		previewChatUC:      previewChatUC,
		createAgentUC:      createAgentUC,
		listAgentsUC:       listAgentsUC,
		updateAgentUC:      updateAgentUC,
		toggleAgentUC:      toggleAgentUC,
		deleteAgentUC:      deleteAgentUC,
		learnPersonalityUC: learnPersonalityUC,
	}
}

type CreateAgentRequest struct {
	Name         string  `json:"name" binding:"required"`
	SystemPrompt *string `json:"system_prompt"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Timezone     string  `json:"timezone"`
	// Mission 7: optional. When both template_id and placeholders
	// are supplied, the server applies placeholders to the template
	// prompt (caller's system_prompt wins if set) and seeds a
	// "Perguntas frequentes" knowledge source with the template's
	// Q/A pairs.
	TemplateID   string            `json:"template_id,omitempty"`
	Placeholders map[string]string `json:"placeholders,omitempty"`
}

// CreateAgent godoc
// @Summary      Create a new AI Agent
// @Description  Initializes a new AI agent for the authenticated account.
// @Tags         Agents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body handlers.CreateAgentRequest true "Agent Creation Payload"
// @Success      201  {object}  domain.Agent
// @Failure      400  {object}  domain.AppError  "Invalid request body"
// @Failure      401  {object}  domain.AppError  "code: AUTH_UNAUTHORIZED — unauthorized"
// @Failure      500  {object}  domain.AppError  "Internal server error"
// @Router       /agents [post]
func (h *AgentHandler) HandleCreateAgent(c *gin.Context) {
	accountID := c.GetString("accountID")
	var req CreateAgentRequest
	if !bindJSON(c, &req) {
		return
	}

	dto := usecases.CreateAgentDTO{
		AccountID:    accountID,
		Name:         req.Name,
		SystemPrompt: req.SystemPrompt,
		Provider:     req.Provider,
		Model:        req.Model,
		Timezone:     req.Timezone,
		TemplateID:   req.TemplateID,
		Placeholders: req.Placeholders,
	}

	agent, err := h.createAgentUC.Execute(c.Request.Context(), dto)
	if err != nil {
		respondError(c, err)
		return
	}

	// Mission 7: if a template was used and we have the knowledge UC
	// wired, seed "Perguntas frequentes". Best-effort — a failure
	// here shouldn't roll back the agent create; the user can still
	// add knowledge manually. Logged with the agent ID so ops can
	// retry from the dashboard if needed.
	if req.TemplateID != "" && h.knowledgeUC != nil {
		if _, err := h.knowledgeUC.SeedFromTemplate(c.Request.Context(), usecases.SeedFromTemplateDTO{
			AccountID:    accountID,
			AgentID:      agent.ID,
			TemplateID:   req.TemplateID,
			Placeholders: req.Placeholders,
			Locale:       AcceptLocale(c),
		}); err != nil {
			slog.Warn("agent template knowledge seed failed",
				"agent_id", agent.ID, "template_id", req.TemplateID, "error", err)
		}
	}

	c.JSON(http.StatusCreated, agent)
}

// ListAgents godoc
// @Summary      List AI Agents
// @Description  Returns all agents associated with the authenticated account.
// @Tags         Agents
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   domain.Agent
// @Failure      401  {object}  domain.AppError  "code: AUTH_UNAUTHORIZED — unauthorized"
// @Failure      500  {object}  domain.AppError  "Internal server error"
// @Router       /agents [get]
func (h *AgentHandler) HandleListAgents(c *gin.Context) {
	accountID := c.GetString("accountID")
	agents, err := h.listAgentsUC.Execute(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to list agents"})
		return
	}
	c.JSON(http.StatusOK, agents)
}

type UpdateAgentRequest struct {
	Name         string  `json:"name" binding:"required"`
	SystemPrompt *string `json:"system_prompt"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Timezone     string  `json:"timezone"`
	// Nilable so that omitting the fields means "don't touch". Days=0
	// explicitly disables the business-hours window (24/7 bot).
	BusinessHoursStart *string `json:"business_hours_start,omitempty"`
	BusinessHoursEnd   *string `json:"business_hours_end,omitempty"`
	BusinessHoursDays  *int    `json:"business_hours_days,omitempty"`
	HideAIIdentity     *bool   `json:"hide_ai_identity,omitempty"`
	SilentOnUnknown    *bool   `json:"silent_on_unknown,omitempty"`
}

// UpdateAgent godoc
// @Summary      Update an AI Agent
// @Description  Updates the name and system prompt of an existing agent.
// @Tags         Agents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Agent ID"
// @Param        request body handlers.UpdateAgentRequest true "Update Payload"
// @Success      200  {object}  domain.Agent
// @Failure      400  {object}  domain.AppError  "Invalid request body"
// @Failure      404  {object}  domain.AppError  "code: AGENT_NOT_FOUND — agent not found"
// @Router       /agents/{id} [put]
func (h *AgentHandler) HandleUpdateAgent(c *gin.Context) {
	agentID := c.Param("id")
	accountID := c.GetString("accountID")
	var req UpdateAgentRequest
	if !bindJSON(c, &req) {
		return
	}

	dto := usecases.UpdateAgentDTO{
		ID:                 agentID,
		AccountID:          accountID,
		Name:               req.Name,
		SystemPrompt:       req.SystemPrompt,
		Provider:           req.Provider,
		Model:              req.Model,
		Timezone:           req.Timezone,
		BusinessHoursStart: req.BusinessHoursStart,
		BusinessHoursEnd:   req.BusinessHoursEnd,
		BusinessHoursDays:  req.BusinessHoursDays,
		HideAIIdentity:     req.HideAIIdentity,
		SilentOnUnknown:    req.SilentOnUnknown,
	}

	agent, err := h.updateAgentUC.Execute(c.Request.Context(), dto)
	if err != nil {
		if err == domain.ErrAgentNotFound {
			c.JSON(http.StatusNotFound, domain.AppError{Code: "NOT_FOUND", Message: "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to update agent"})
		return
	}

	c.JSON(http.StatusOK, agent)
}

type ToggleAgentRequest struct {
	IsActive bool `json:"is_active"`
}

// PauseAgent godoc
// @Summary      Toggle AI Agent Status
// @Description  Enables or Disables an AI agent (pausing its interactions).
// @Tags         Agents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Agent ID"
// @Param        request body handlers.ToggleAgentRequest true "Status Payload"
// @Success      200  {object}  map[string]string  "Status updated"
// @Failure      404  {object}  domain.AppError  "code: AGENT_NOT_FOUND — agent not found"
// @Router       /agents/{id}/toggle [patch]
func (h *AgentHandler) HandlePauseAgent(c *gin.Context) {
	agentID := c.Param("id")
	accountID := c.GetString("accountID")
	var req ToggleAgentRequest
	if !bindJSON(c, &req) {
		return
	}

	err := h.toggleAgentUC.Execute(c.Request.Context(), accountID, agentID, req.IsActive)
	if err != nil {
		if err == domain.ErrAgentNotFound {
			c.JSON(http.StatusNotFound, domain.AppError{Code: "NOT_FOUND", Message: "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to toggle status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated", "is_active": req.IsActive})
}

// DeleteAgent godoc
// @Summary      Delete an AI Agent
// @Description  Permanently deletes an AI agent. Only the owning account can delete it.
// @Tags         Agents
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Agent ID"
// @Success      200  {object}  map[string]string  "Agent deleted successfully"
// @Failure      404  {object}  domain.AppError  "code: AGENT_NOT_FOUND — agent not found or unauthorized"
// @Failure      500  {object}  domain.AppError  "Internal server error"
// @Router       /agents/{id} [delete]
func (h *AgentHandler) HandleDeleteAgent(c *gin.Context) {
	agentID := c.Param("id")
	accountID := c.GetString("accountID")

	err := h.deleteAgentUC.Execute(c.Request.Context(), accountID, agentID)
	if err != nil {
		if err == domain.ErrAgentNotFound {
			c.JSON(http.StatusNotFound, domain.AppError{Code: "NOT_FOUND", Message: "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to delete agent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "agent deleted successfully"})
}

// HandlePreviewChatRequest defines the input payload for the endpoint
type HandlePreviewChatRequest struct {
	AgentID string `json:"agent_id" binding:"required"`
	Message string `json:"message" binding:"required"`
}

// PreviewChat godoc
// @Summary      Preview AI Agent Chat
// @Description  Allows interacting with an AI Agent for testing/preview purposes.
// @Tags         Agents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body handlers.HandlePreviewChatRequest true "Preview Chat Request"
// @Success      200  {object}  usecases.PreviewChatResponse
// @Failure      400  {object}  domain.AppError  "code: USAGE_INSUFFICIENT_TOKENS — insufficient AI tokens | USAGE_NO_CREDITS — AI limits reached"
// @Failure      401  {object}  domain.AppError  "code: AUTH_UNAUTHORIZED — unauthorized"
// @Failure      500  {object}  domain.AppError  "Internal server error"
// @Router       /agents/preview [post]
func (h *AgentHandler) HandlePreviewChat(c *gin.Context) {
	// Extract accountID from the JWT context
	accountID := c.GetString("accountID")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req HandlePreviewChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	dto := usecases.PreviewChatDTO{
		AccountID: accountID,
		AgentID:   req.AgentID,
		Message:   req.Message,
	}

	response, err := h.previewChatUC.Execute(c.Request.Context(), dto)
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			// Log AppError path too so the upstream reason (e.g. the
			// raw error from an AI provider wrapped by
			// AI_PROVIDER_UNAVAILABLE) shows up in `docker logs`.
			// Without this we only see the request line's 400 status
			// and have to dig into DevTools for the body.
			slog.Warn("preview chat rejected",
				"code", appErr.Code,
				"message", appErr.Message,
				"account_id", accountID,
				"agent_id", req.AgentID)
			c.JSON(http.StatusBadRequest, gin.H{"error": appErr.Message, "code": appErr.Code})
			return
		}
		slog.Error("preview chat failed",
			"error", err.Error(),
			"account_id", accountID,
			"agent_id", req.AgentID)
		// Surface the upstream error message on the 500 body so users
		// can screenshot it and ops can diagnose without scraping
		// container logs. The handler already rejects unauthenticated
		// requests above, so no PII from other tenants leaks here.
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleLearnPersonality analyzes human agent messages and generates a personality prompt.
func (h *AgentHandler) HandleLearnPersonality(c *gin.Context) {
	accountID := c.GetString("accountID")
	agentID := c.Param("id")

	agent, err := h.learnPersonalityUC.Execute(c.Request.Context(), agentID, accountID)
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			c.JSON(http.StatusBadRequest, appErr)
			return
		}
		slog.Error("learn personality failed", "agent_id", agentID, "error", err)
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "falha ao aprender personalidade"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":            "Personalidade aprendida com sucesso!",
		"personality_prompt": agent.PersonalityPrompt,
	})
}
