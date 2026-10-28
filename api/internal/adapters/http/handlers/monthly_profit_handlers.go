package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/usecases"
)

// Monthly-profit management (US-006, vendedores v3) lives on the
// OwnerHandler — same /owner group, same RequireSuperAdmin gate. Same
// setter pattern as the provider-cost endpoints: NewOwnerHandler stays
// stable, routes 503 when the use case was never wired.

// SetMonthlyProfitUC wires the monthly-profit use case.
func (h *OwnerHandler) SetMonthlyProfitUC(uc *usecases.ComputeMonthlyProfitUseCase) {
	h.profitUC = uc
}

func (h *OwnerHandler) profitUCOrAbort(c *gin.Context) *usecases.ComputeMonthlyProfitUseCase {
	if h.profitUC == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "monthly profit is not configured on this deployment"})
		return nil
	}
	return h.profitUC
}

// @Summary      List a cycle's per-account net profit rows
// @Tags         owner
// @Produce      json
// @Param        cycle query string true "Cycle YYYY-MM"
// @Success      200 {object} usecases.ProfitCycleListing
// @Security     BearerAuth
// @Router       /owner/monthly-profit [get]
func (h *OwnerHandler) HandleListMonthlyProfit(c *gin.Context) {
	uc := h.profitUCOrAbort(c)
	if uc == nil {
		return
	}
	out, err := uc.List(c.Request.Context(), c.Query("cycle"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type recomputeProfitReq struct {
	// Cycle is "YYYY-MM".
	Cycle string `json:"cycle" binding:"required"`
	// AccountID narrows the recompute to one account; empty recomputes
	// every seller-linked account.
	AccountID string `json:"account_id"`
}

// @Summary      Recompute a cycle's net profit (idempotent upsert)
// @Tags         owner
// @Accept       json
// @Produce      json
// @Param        body body recomputeProfitReq true "Cycle + optional account"
// @Success      200 {object} usecases.ProfitCycleListing
// @Security     BearerAuth
// @Router       /owner/monthly-profit/recompute [post]
func (h *OwnerHandler) HandleRecomputeMonthlyProfit(c *gin.Context) {
	uc := h.profitUCOrAbort(c)
	if uc == nil {
		return
	}
	var req recomputeProfitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cycle is required (YYYY-MM)"})
		return
	}
	if _, err := uc.RecomputeCycle(c.Request.Context(), c.GetString("userEmail"), req.Cycle, req.AccountID); err != nil {
		respondError(c, err)
		return
	}
	// Return the freshly persisted listing so the panel refreshes in
	// one roundtrip.
	out, err := uc.List(c.Request.Context(), req.Cycle)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type profitSettingsReq struct {
	// TaxBps is the tax on revenue in basis points (600 = 6%).
	TaxBps int `json:"tax_bps"`
	// InfraCostCents is the flat infra cost per account per month in
	// BRL cents.
	InfraCostCents int `json:"infra_cost_cents"`
}

// @Summary      Set profit-computation settings (tax bps + infra cost)
// @Tags         owner
// @Accept       json
// @Produce      json
// @Param        body body profitSettingsReq true "Settings"
// @Success      200 {object} map[string]string
// @Security     BearerAuth
// @Router       /owner/profit-settings [put]
func (h *OwnerHandler) HandleSetProfitSettings(c *gin.Context) {
	uc := h.profitUCOrAbort(c)
	if uc == nil {
		return
	}
	var req profitSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := uc.SetProfitSettings(c.Request.Context(), c.GetString("userEmail"), req.TaxBps, req.InfraCostCents); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
