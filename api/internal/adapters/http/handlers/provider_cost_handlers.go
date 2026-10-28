package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Provider-cost management (US-005, vendedores v3) lives on the
// OwnerHandler — same /owner group, same RequireSuperAdmin gate. The
// use case is wired via SetProviderCostUC (setter pattern keeps
// NewOwnerHandler stable); routes 503 when it was never wired.

// SetProviderCostUC wires the provider-cost admin use case.
func (h *OwnerHandler) SetProviderCostUC(uc *usecases.ProviderCostAdminUseCase) {
	h.costUC = uc
}

func (h *OwnerHandler) costUCOrAbort(c *gin.Context) *usecases.ProviderCostAdminUseCase {
	if h.costUC == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provider costs are not configured on this deployment"})
		return nil
	}
	return h.costUC
}

// @Summary      List provider token costs + USD->BRL rate
// @Tags         owner
// @Produce      json
// @Success      200 {object} usecases.ProviderCostListing
// @Security     BearerAuth
// @Router       /owner/provider-costs [get]
func (h *OwnerHandler) HandleListProviderCosts(c *gin.Context) {
	uc := h.costUCOrAbort(c)
	if uc == nil {
		return
	}
	out, err := uc.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type providerCostReq struct {
	Provider                string `json:"provider"`
	Model                   string `json:"model"`
	InputCostPer1MCentsUSD  int64  `json:"input_cost_per_1m_cents_usd"`
	OutputCostPer1MCentsUSD int64  `json:"output_cost_per_1m_cents_usd"`
	// EffectiveFrom is "YYYY-MM-DD".
	EffectiveFrom string `json:"effective_from" binding:"required"`
}

// @Summary      Add a provider token cost row (price changes = new row)
// @Tags         owner
// @Accept       json
// @Produce      json
// @Success      201 {object} domain.ProviderCost
// @Security     BearerAuth
// @Router       /owner/provider-costs [post]
func (h *OwnerHandler) HandleCreateProviderCost(c *gin.Context) {
	uc := h.costUCOrAbort(c)
	if uc == nil {
		return
	}
	var req providerCostReq
	if !bindJSON(c, &req) {
		return
	}
	var out *domain.ProviderCost
	out, err := uc.Create(c.Request.Context(), c.GetString("userEmail"), usecases.ProviderCostDTO{
		Provider:                req.Provider,
		Model:                   req.Model,
		InputCostPer1MCentsUSD:  req.InputCostPer1MCentsUSD,
		OutputCostPer1MCentsUSD: req.OutputCostPer1MCentsUSD,
		EffectiveFrom:           req.EffectiveFrom,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

// @Summary      Fix a provider cost row in place (costs + effective_from)
// @Tags         owner
// @Accept       json
// @Produce      json
// @Success      200 {object} map[string]bool
// @Security     BearerAuth
// @Router       /owner/provider-costs/{id} [put]
func (h *OwnerHandler) HandleUpdateProviderCost(c *gin.Context) {
	uc := h.costUCOrAbort(c)
	if uc == nil {
		return
	}
	var req providerCostReq
	if !bindJSON(c, &req) {
		return
	}
	err := uc.Update(c.Request.Context(), c.GetString("userEmail"), c.Param("id"), usecases.ProviderCostDTO{
		InputCostPer1MCentsUSD:  req.InputCostPer1MCentsUSD,
		OutputCostPer1MCentsUSD: req.OutputCostPer1MCentsUSD,
		EffectiveFrom:           req.EffectiveFrom,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// @Summary      Delete a provider cost row
// @Tags         owner
// @Produce      json
// @Success      200 {object} map[string]bool
// @Security     BearerAuth
// @Router       /owner/provider-costs/{id} [delete]
func (h *OwnerHandler) HandleDeleteProviderCost(c *gin.Context) {
	uc := h.costUCOrAbort(c)
	if uc == nil {
		return
	}
	if err := uc.Delete(c.Request.Context(), c.GetString("userEmail"), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type setExchangeRateReq struct {
	// USDBRLRateCents is BRL cents per 1 USD (550 = R$5.50).
	USDBRLRateCents int `json:"usd_brl_rate_cents" binding:"required"`
}

// @Summary      Set the manual USD->BRL rate used by profit computation
// @Tags         owner
// @Accept       json
// @Produce      json
// @Success      200 {object} map[string]int
// @Security     BearerAuth
// @Router       /owner/exchange-rate [put]
func (h *OwnerHandler) HandleSetExchangeRate(c *gin.Context) {
	uc := h.costUCOrAbort(c)
	if uc == nil {
		return
	}
	var req setExchangeRateReq
	if !bindJSON(c, &req) {
		return
	}
	if err := uc.SetUSDBRLRateCents(c.Request.Context(), c.GetString("userEmail"), req.USDBRLRateCents); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "usd_brl_rate_cents": req.USDBRLRateCents})
}
