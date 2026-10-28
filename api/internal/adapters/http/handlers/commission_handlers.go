package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Commission-ledger management (US-007, vendedores v3) lives on the
// OwnerHandler — same /owner group, same RequireSuperAdmin gate. Same
// setter pattern as the monthly-profit endpoints: NewOwnerHandler stays
// stable, routes 503 when the use case was never wired.

// SetCommissionUC wires the commission use case.
func (h *OwnerHandler) SetCommissionUC(uc *usecases.ComputeCommissionsUseCase) {
	h.commissionUC = uc
}

func (h *OwnerHandler) commissionUCOrAbort(c *gin.Context) *usecases.ComputeCommissionsUseCase {
	if h.commissionUC == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "commissions are not configured on this deployment"})
		return nil
	}
	return h.commissionUC
}

// @Summary      List a cycle's commission ledger
// @Tags         owner
// @Produce      json
// @Param        cycle query string true "Cycle YYYY-MM"
// @Success      200 {object} usecases.CommissionCycleListing
// @Security     BearerAuth
// @Router       /owner/commissions [get]
func (h *OwnerHandler) HandleListCommissions(c *gin.Context) {
	uc := h.commissionUCOrAbort(c)
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

type recomputeCommissionsReq struct {
	// Cycle is "YYYY-MM".
	Cycle string `json:"cycle" binding:"required"`
}

// @Summary      Recompute a cycle's commissions from the profit table (idempotent upsert; paid entries untouched)
// @Tags         owner
// @Accept       json
// @Produce      json
// @Param        body body recomputeCommissionsReq true "Cycle"
// @Success      200 {object} usecases.CommissionCycleListing
// @Security     BearerAuth
// @Router       /owner/commissions/recompute [post]
func (h *OwnerHandler) HandleRecomputeCommissions(c *gin.Context) {
	uc := h.commissionUCOrAbort(c)
	if uc == nil {
		return
	}
	var req recomputeCommissionsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cycle is required (YYYY-MM)"})
		return
	}
	if _, err := uc.ComputeCycle(c.Request.Context(), c.GetString("userEmail"), req.Cycle); err != nil {
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

type setCommissionSettingsReq struct {
	// BonusNPSCents is the fixed promoter-NPS bonus (US-008), integer
	// BRL cents; 0 disables it.
	BonusNPSCents *int64 `json:"bonus_nps_cents" binding:"required"`
	// BonusRetentionCents is the one-time 12-active-months bonus,
	// integer BRL cents; 0 disables it.
	BonusRetentionCents *int64 `json:"bonus_retention_cents" binding:"required"`
}

// @Summary      Set the objective-event bonus amounts (US-008: promoter NPS + 12-month retention)
// @Tags         owner
// @Accept       json
// @Produce      json
// @Param        body body setCommissionSettingsReq true "Bonus amounts in BRL cents"
// @Success      204
// @Security     BearerAuth
// @Router       /owner/commission-settings [put]
func (h *OwnerHandler) HandleSetCommissionSettings(c *gin.Context) {
	uc := h.commissionUCOrAbort(c)
	if uc == nil {
		return
	}
	var req setCommissionSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil || req.BonusNPSCents == nil || req.BonusRetentionCents == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bonus_nps_cents and bonus_retention_cents are required"})
		return
	}
	if err := uc.SetBonusSettings(c.Request.Context(), c.GetString("userEmail"), *req.BonusNPSCents, *req.BonusRetentionCents); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type setSLASettingsReq struct {
	// FirstResponseMinutes is the SLA threshold in business minutes
	// (default 240 = 4 business hours).
	FirstResponseMinutes int64 `json:"first_response_minutes" binding:"required"`
	// BreachPct: the cycle breaches when MORE than this % of the
	// seller's assigned conversations blow the threshold.
	BreachPct int64 `json:"breach_pct" binding:"required"`
	// Business window hours [start, end) in Timezone; weekends are
	// always excluded.
	BusinessStartHour int64  `json:"business_start_hour"`
	BusinessEndHour   int64  `json:"business_end_hour" binding:"required"`
	Timezone          string `json:"timezone" binding:"required"`
}

// @Summary      Set the SLA ladder parameters (US-009: first-response threshold, breach cut, business window)
// @Tags         owner
// @Accept       json
// @Produce      json
// @Param        body body setSLASettingsReq true "SLA parameters"
// @Success      204
// @Security     BearerAuth
// @Router       /owner/sla-settings [put]
func (h *OwnerHandler) HandleSetSLASettings(c *gin.Context) {
	uc := h.commissionUCOrAbort(c)
	if uc == nil {
		return
	}
	var req setSLASettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "first_response_minutes, breach_pct, business_start_hour, business_end_hour and timezone are required"})
		return
	}
	err := uc.SetSLASettings(c.Request.Context(), c.GetString("userEmail"), usecases.SLASettings{
		FirstResponseMinutes: req.FirstResponseMinutes,
		BreachPct:            req.BreachPct,
		BusinessStartHour:    req.BusinessStartHour,
		BusinessEndHour:      req.BusinessEndHour,
		Timezone:             req.Timezone,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary      List a cycle's payout summaries grouped per seller (US-013: pending + earlier-cycle adjustments + PIX key)
// @Tags         owner
// @Produce      json
// @Param        cycle query string true "Cycle YYYY-MM"
// @Success      200 {object} usecases.PayoutCycleListing
// @Security     BearerAuth
// @Router       /owner/payouts [get]
func (h *OwnerHandler) HandleListPayouts(c *gin.Context) {
	uc := h.commissionUCOrAbort(c)
	if uc == nil {
		return
	}
	out, err := uc.ListPayouts(c.Request.Context(), c.Query("cycle"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type markPayoutPaidReq struct {
	SellerUserID string `json:"seller_user_id" binding:"required"`
	// Cycle is "YYYY-MM"; pending entries of EARLIER cycles
	// (adjustments) settle together.
	Cycle string `json:"cycle" binding:"required"`
	// Note is the optional PIX transaction reference stamped on every
	// settled entry.
	Note string `json:"note"`
	// AllowOpenCycle is the explicit override to settle the RUNNING
	// cycle (partial payout) — without it, an open/future cycle is
	// rejected with COMMISSION_CYCLE_OPEN (US-005).
	AllowOpenCycle bool `json:"allow_open_cycle"`
}

// @Summary      Mark a seller's payout as paid through the cycle (idempotent — only pending entries flip)
// @Tags         owner
// @Accept       json
// @Produce      json
// @Param        body body markPayoutPaidReq true "Seller, cycle and optional PIX note"
// @Success      200 {object} usecases.PayoutCycleListing
// @Security     BearerAuth
// @Router       /owner/payouts/mark-paid [post]
func (h *OwnerHandler) HandleMarkPayoutPaid(c *gin.Context) {
	uc := h.commissionUCOrAbort(c)
	if uc == nil {
		return
	}
	var req markPayoutPaidReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "seller_user_id and cycle (YYYY-MM) are required"})
		return
	}
	out, err := uc.MarkPayoutPaid(c.Request.Context(), c.GetString("userEmail"), req.SellerUserID, req.Cycle, req.Note, req.AllowOpenCycle)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// @Summary      Export a cycle's full commission ledger as CSV (accounting)
// @Tags         owner
// @Produce      text/csv
// @Param        cycle query string true "Cycle YYYY-MM"
// @Success      200 {string} string "CSV"
// @Security     BearerAuth
// @Router       /owner/payouts/export [get]
func (h *OwnerHandler) HandleExportPayoutCSV(c *gin.Context) {
	uc := h.commissionUCOrAbort(c)
	if uc == nil {
		return
	}
	cycle := c.Query("cycle")
	data, err := uc.ExportPayoutCSV(c.Request.Context(), cycle)
	if err != nil {
		respondError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="payouts-`+cycle+`.csv"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", data)
}

type setSellerActivityReq struct {
	IsActive *bool `json:"is_active" binding:"required"`
}

// @Summary      Set a seller's activity clause (deactivating starts the 12-month 15% residual window)
// @Tags         owner
// @Accept       json
// @Produce      json
// @Param        id path string true "Seller user ID"
// @Param        body body setSellerActivityReq true "Activity state"
// @Success      200 {object} domain.SellerSettings
// @Security     BearerAuth
// @Router       /owner/sellers/{id}/activity [put]
func (h *OwnerHandler) HandleSetSellerActivity(c *gin.Context) {
	uc := h.commissionUCOrAbort(c)
	if uc == nil {
		return
	}
	var req setSellerActivityReq
	if err := c.ShouldBindJSON(&req); err != nil || req.IsActive == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "is_active (bool) is required"})
		return
	}
	var out *domain.SellerSettings
	out, err := uc.SetSellerActivity(c.Request.Context(), c.GetString("userEmail"), c.Param("id"), *req.IsActive)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type setSellerFlagReq struct {
	IsSeller *bool `json:"is_seller" binding:"required"`
}

// @Summary      Grant or revoke a user's explicit seller flag (US-014 — only flagged users reach the /seller panel or become payable)
// @Tags         owner
// @Accept       json
// @Produce      json
// @Param        id path string true "Seller user ID"
// @Param        body body setSellerFlagReq true "Seller flag"
// @Success      200 {object} domain.SellerSettings
// @Security     BearerAuth
// @Router       /owner/sellers/{id}/flag [put]
func (h *OwnerHandler) HandleSetSellerFlag(c *gin.Context) {
	uc := h.commissionUCOrAbort(c)
	if uc == nil {
		return
	}
	var req setSellerFlagReq
	if err := c.ShouldBindJSON(&req); err != nil || req.IsSeller == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "is_seller (bool) is required"})
		return
	}
	out, err := uc.SetSellerFlag(c.Request.Context(), c.GetString("userEmail"), c.Param("id"), *req.IsSeller)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}
