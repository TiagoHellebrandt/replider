package router

import (
	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
)

// setupOwnerRoutes wires the platform-level admin endpoints. The route
// group already has authMw + RequireSuperAdmin applied upstream — these
// routes never reach without a valid JWT belonging to an email on the
// SUPER_ADMIN_EMAILS allowlist.
func setupOwnerRoutes(v1 *gin.RouterGroup, h *handlers.OwnerHandler, mws ...gin.HandlerFunc) {
	owner := v1.Group("/owner")
	owner.Use(mws...)
	{
		owner.GET("/system-health", h.HandleSystemHealth)
		// Account invites (sales-led signup): the one-shot permission that
		// passes a closed registration door. The unfiltered plans listing
		// lives here because the public /plans hides the SDR tiers an
		// invite points at.
		owner.POST("/invites", h.HandleCreateInvite)
		owner.GET("/invites", h.HandleListInvites)
		owner.POST("/invites/:id/revoke", h.HandleRevokeInvite)
		owner.GET("/plans", h.HandleListAllPlans)
		owner.GET("/opportunity-report", h.HandleOpportunityReport)
		owner.GET("/accounts", h.HandleListAccounts)
		owner.GET("/accounts/top", h.HandleTopAccounts)
		owner.POST("/accounts", h.HandleCreateContract)
		owner.GET("/accounts/:id", h.HandleGetAccount)
		owner.PATCH("/accounts/:id/overrides", h.HandleSetOverrides)
		owner.POST("/accounts/:id/suspend", h.HandleSuspend)
		owner.POST("/accounts/:id/reactivate", h.HandleReactivate)
		owner.PATCH("/accounts/:id/subscription-status", h.HandleSetStatus)
		owner.PUT("/accounts/:id/currency", h.HandleSetCurrency)
		owner.PATCH("/accounts/:id/apply-contract", h.HandleApplyContract)
		owner.PUT("/accounts/:id/seller", h.HandleSetSeller)
		owner.POST("/accounts/:id/grant-credits", h.HandleGrantCredits)
		owner.POST("/accounts/:id/extend-trial", h.HandleExtendTrial)
		owner.POST("/accounts/:id/impersonate", h.HandleImpersonate)
		owner.GET("/accounts/:id/conversations", h.HandleListConversations)
		owner.GET("/accounts/:id/usage-trend", h.HandleUsageTrend)
		owner.GET("/accounts/:id/invoices", h.HandleListInvoices)
		// Provider token costs + manual USD->BRL rate (US-005,
		// vendedores v3) — inputs of the monthly profit computation.
		owner.GET("/provider-costs", h.HandleListProviderCosts)
		owner.POST("/provider-costs", h.HandleCreateProviderCost)
		owner.PUT("/provider-costs/:id", h.HandleUpdateProviderCost)
		owner.DELETE("/provider-costs/:id", h.HandleDeleteProviderCost)
		owner.PUT("/exchange-rate", h.HandleSetExchangeRate)
		// Per-account monthly net profit — the commission base
		// (US-006, vendedores v3). Recompute is an idempotent upsert.
		owner.GET("/monthly-profit", h.HandleListMonthlyProfit)
		owner.POST("/monthly-profit/recompute", h.HandleRecomputeMonthlyProfit)
		owner.PUT("/profit-settings", h.HandleSetProfitSettings)
		// Commission ledger — tracks 50/20 + activity clause (US-007,
		// vendedores v3). Recompute never touches paid entries.
		owner.GET("/commissions", h.HandleListCommissions)
		owner.POST("/commissions/recompute", h.HandleRecomputeCommissions)
		// Objective-event bonus amounts (US-008): promoter NPS +
		// 12-month retention, owner-editable.
		owner.PUT("/commission-settings", h.HandleSetCommissionSettings)
		// SLA ladder parameters (US-009): first-response threshold,
		// breach cut and business window.
		owner.PUT("/sla-settings", h.HandleSetSLASettings)
		// Vendedores section (owner-sellers US-001): unified list +
		// atomic onboard (flag + ref code + rotation in one tx).
		owner.GET("/sellers", h.HandleListSellers)
		owner.PUT("/sellers/:id/onboard", h.HandleOnboardSeller)
		owner.PUT("/sellers/:id/activity", h.HandleSetSellerActivity)
		// Explicit seller grant (US-014): only flagged users reach the
		// /seller panel or become payable.
		owner.PUT("/sellers/:id/flag", h.HandleSetSellerFlag)
		// Manual PIX payout (US-013): per-seller totals + mark paid
		// (idempotent) + CSV export for accounting.
		owner.GET("/payouts", h.HandleListPayouts)
		owner.POST("/payouts/mark-paid", h.HandleMarkPayoutPaid)
		owner.GET("/payouts/export", h.HandleExportPayoutCSV)
	}
}
