package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// FunnelHandler exposes sales-funnel endpoints (outbound sales module):
// AI persona generation (US-005) and account-scoped funnel CRUD (US-006).
// All routes are mounted under admin + paid middleware; account scoping is
// enforced by the use cases via the accountID claim.
type FunnelHandler struct {
	generatePersonaUC *usecases.GeneratePersonaUseCase
	funnelUC          *usecases.FunnelUseCase
	// prospectingUC is optional (wired via SetProspectingUC). When unset the
	// prospecting endpoint returns 503 PROSPECTING_NOT_CONFIGURED.
	prospectingUC *usecases.ProspectingUseCase
}

func NewFunnelHandler(generatePersonaUC *usecases.GeneratePersonaUseCase, funnelUC *usecases.FunnelUseCase) *FunnelHandler {
	return &FunnelHandler{generatePersonaUC: generatePersonaUC, funnelUC: funnelUC}
}

type generatePersonaReq struct {
	ProductID   string `json:"product_id"`
	Description string `json:"description"`
}

// HandleGeneratePersona generates a consultative sales persona prompt from
// a product (or a free-form description) using the account's AI/BYOK key.
func (h *FunnelHandler) HandleGeneratePersona(c *gin.Context) {
	accountID := getAccountID(c)
	var req generatePersonaReq
	if !bindJSON(c, &req) {
		return
	}

	res, err := h.generatePersonaUC.Execute(c.Request.Context(), usecases.GeneratePersonaDTO{
		AccountID:   accountID,
		ProductID:   req.ProductID,
		Description: req.Description,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

type funnelReq struct {
	Name                      string          `json:"name" binding:"required"`
	ProductID                 *string         `json:"product_id"`
	AgentID                   *string         `json:"agent_id"`
	ChannelID                 *string         `json:"channel_id"`
	PersonaPrompt             string          `json:"persona_prompt"`
	MetadataSchema            json.RawMessage `json:"metadata_schema"`
	WebhookURL                string          `json:"webhook_url"`
	WebhookSecret             string          `json:"webhook_secret"`
	TriggerOnRequiredComplete bool            `json:"trigger_on_required_complete"`
	TriggerOnBuyIntent        bool            `json:"trigger_on_buy_intent"`
	Active                    *bool           `json:"active"`
}

func (h *FunnelHandler) HandleCreate(c *gin.Context) {
	accountID := getAccountID(c)
	var req funnelReq
	if !bindJSON(c, &req) {
		return
	}

	f, err := h.funnelUC.Create(c.Request.Context(), usecases.CreateFunnelDTO{
		AccountID:                 accountID,
		Name:                      req.Name,
		ProductID:                 req.ProductID,
		AgentID:                   req.AgentID,
		ChannelID:                 req.ChannelID,
		PersonaPrompt:             req.PersonaPrompt,
		MetadataSchema:            req.MetadataSchema,
		WebhookURL:                req.WebhookURL,
		WebhookSecret:             req.WebhookSecret,
		TriggerOnRequiredComplete: req.TriggerOnRequiredComplete,
		TriggerOnBuyIntent:        req.TriggerOnBuyIntent,
		Active:                    req.Active,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, f)
}

func (h *FunnelHandler) HandleList(c *gin.Context) {
	accountID := getAccountID(c)
	funnels, err := h.funnelUC.List(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, err)
		return
	}
	// The list is readable by every lead actor (the Clientes tab needs a
	// funnel id before it can draw anything), but the webhook secret is an
	// integration credential — admin material. Blank it for everyone else
	// rather than growing a second DTO.
	if !domain.RoleSatisfies(domain.NormalizeRole(getUserRole(c)), domain.RoleAdminRole) {
		for _, f := range funnels {
			f.WebhookSecret = ""
		}
	}
	c.JSON(http.StatusOK, funnels)
}

func (h *FunnelHandler) HandleGet(c *gin.Context) {
	accountID := getAccountID(c)
	f, err := h.funnelUC.Get(c.Request.Context(), accountID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, f)
}

func (h *FunnelHandler) HandleUpdate(c *gin.Context) {
	accountID := getAccountID(c)
	var req funnelReq
	if !bindJSON(c, &req) {
		return
	}

	f, err := h.funnelUC.Update(c.Request.Context(), usecases.UpdateFunnelDTO{
		AccountID:                 accountID,
		ID:                        c.Param("id"),
		Name:                      req.Name,
		ProductID:                 req.ProductID,
		AgentID:                   req.AgentID,
		ChannelID:                 req.ChannelID,
		PersonaPrompt:             req.PersonaPrompt,
		MetadataSchema:            req.MetadataSchema,
		WebhookURL:                req.WebhookURL,
		WebhookSecret:             req.WebhookSecret,
		TriggerOnRequiredComplete: req.TriggerOnRequiredComplete,
		TriggerOnBuyIntent:        req.TriggerOnBuyIntent,
		Active:                    req.Active,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, f)
}

// HandleTestWebhook fires a synthetic signed event at the funnel's configured
// webhook and returns the consumer's HTTP status + response body (US-017). Used
// by the "Enviar evento de teste" button on the funnel webhook config (US-010).
func (h *FunnelHandler) HandleTestWebhook(c *gin.Context) {
	accountID := getAccountID(c)
	res, err := h.funnelUC.TestWebhook(c.Request.Context(), accountID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *FunnelHandler) HandleDelete(c *gin.Context) {
	accountID := getAccountID(c)
	if err := h.funnelUC.Delete(c.Request.Context(), accountID, c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "funnel deleted"})
}
