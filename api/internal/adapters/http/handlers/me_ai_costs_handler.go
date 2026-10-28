package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// AI cost surface (BYOK cost model): what the account spent, what a month
// looks like at its own pace, and the customer-set ceiling. Rides MeHandler
// via optional setter, same pattern as the dashboard BFF.

// SetAICostsUC wires the report (nil → 503 on the routes).
func (h *MeHandler) SetAICostsUC(uc *usecases.AICostReportUseCase) {
	h.aiCostsUC = uc
}

// HandleGetAICosts serves GET /v1/me/ai-costs (admin).
//
// @Summary      AI spend: current cycle, 30-day projection and the account ceiling
// @Tags         Billing
// @Produce      json
// @Success      200 {object} usecases.AICostReport
// @Security     BearerAuth
// @Router       /me/ai-costs [get]
func (h *MeHandler) HandleGetAICosts(c *gin.Context) {
	if h.aiCostsUC == nil {
		respondErr(c, &domain.AppError{Code: "AI_COSTS_UNAVAILABLE", Message: "AI cost report not configured"})
		return
	}
	report, err := h.aiCostsUC.Execute(c.Request.Context(), c.GetString("accountID"))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, report)
}

type setAIBudgetRequest struct {
	// BudgetCents is the monthly ceiling in BRL cents; 0 or negative clears
	// it (no ceiling).
	BudgetCents int64 `json:"budget_cents"`
}

// HandleSetAIBudget serves PUT /v1/me/ai-budget (admin).
//
// @Summary      Set the account's own monthly AI spending ceiling
// @Tags         Billing
// @Accept       json
// @Produce      json
// @Success      200 {object} usecases.AICostReport
// @Failure      400 {object} domain.AppError
// @Security     BearerAuth
// @Router       /me/ai-budget [put]
func (h *MeHandler) HandleSetAIBudget(c *gin.Context) {
	if h.aiCostsUC == nil {
		respondErr(c, &domain.AppError{Code: "AI_COSTS_UNAVAILABLE", Message: "AI cost report not configured"})
		return
	}
	var req setAIBudgetRequest
	if !bindJSON(c, &req) {
		return
	}
	cents := req.BudgetCents
	if err := h.aiCostsUC.SetBudget(c.Request.Context(), c.GetString("accountID"), &cents); err != nil {
		respondErr(c, err)
		return
	}
	h.HandleGetAICosts(c)
}
