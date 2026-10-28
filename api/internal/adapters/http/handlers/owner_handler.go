package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// OwnerHandler exposes the platform-level admin endpoints under
// /v1/owner. The route group is gated by RequireSuperAdmin upstream, so
// these handlers do not need to re-check permissions — they only marshal
// the request and translate domain errors to HTTP responses.
type OwnerHandler struct {
	uc     *usecases.OwnerAdminUseCase
	jwtSvc domain.JWTService
	// costUC serves the provider-cost + exchange-rate endpoints
	// (US-005). Wired via SetProviderCostUC; nil → those routes 503.
	costUC *usecases.ProviderCostAdminUseCase
	// profitUC serves the monthly-profit endpoints (US-006). Wired via
	// SetMonthlyProfitUC; nil → those routes 503.
	profitUC *usecases.ComputeMonthlyProfitUseCase
	// inviteUC serves the account-invite endpoints (sales-led signup).
	// Wired via SetInviteUC; nil → those routes 503.
	inviteUC *usecases.InviteUseCase
	// opportunityReportUC serves the prospect lost-opportunity report
	// (tese-agente-autonomo §4). Wired via SetOpportunityReportUC.
	opportunityReportUC *usecases.OpportunityReportUseCase
	// commissionUC serves the commission-ledger endpoints (US-007).
	// Wired via SetCommissionUC; nil → those routes 503.
	commissionUC *usecases.ComputeCommissionsUseCase
	// ownerSellersUC serves the Vendedores section (owner-sellers
	// US-001). Wired via SetOwnerSellersUC; nil → those routes 503.
	ownerSellersUC *usecases.OwnerSellersUseCase
}

func NewOwnerHandler(uc *usecases.OwnerAdminUseCase, jwtSvc domain.JWTService) *OwnerHandler {
	return &OwnerHandler{uc: uc, jwtSvc: jwtSvc}
}

// HandleListAccounts paginates accounts. Query params: search, status, limit, offset.
func (h *OwnerHandler) HandleListAccounts(c *gin.Context) {
	search := c.Query("search")
	status := c.Query("status")
	limit, offset := pagination(c)
	rows, total, err := h.uc.List(c.Request.Context(), search, status, limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items": rows,
		"total": total,
	})
}

func (h *OwnerHandler) HandleGetAccount(c *gin.Context) {
	out, err := h.uc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type setOverridesReq struct {
	MaxAgents                    *int `json:"max_agents"`
	MaxChannels                  *int `json:"max_channels"`
	MaxSeats                     *int `json:"max_seats"`
	MaxConversationsPerMonth     *int `json:"max_conversations_per_month"`
	MaxStorageMB                 *int `json:"max_storage_mb"`
	MaxAITokensPerMonth          *int `json:"max_ai_tokens_per_month"`
	MaxBroadcastMessagesPerMonth *int `json:"max_broadcast_messages_per_month"`
	MaxRecipientsPerCampaign     *int `json:"max_recipients_per_campaign"`
}

func (h *OwnerHandler) HandleSetOverrides(c *gin.Context) {
	var req setOverridesReq
	if !bindJSON(c, &req) {
		return
	}
	err := h.uc.SetOverrides(c.Request.Context(), c.GetString("userEmail"), c.Param("id"), domain.AccountLimitOverrides{
		MaxAgents:                    req.MaxAgents,
		MaxChannels:                  req.MaxChannels,
		MaxSeats:                     req.MaxSeats,
		MaxConversationsPerMonth:     req.MaxConversationsPerMonth,
		MaxStorageMB:                 req.MaxStorageMB,
		MaxAITokensPerMonth:          req.MaxAITokensPerMonth,
		MaxBroadcastMessagesPerMonth: req.MaxBroadcastMessagesPerMonth,
		MaxRecipientsPerCampaign:     req.MaxRecipientsPerCampaign,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type suspendReq struct {
	Reason string `json:"reason" binding:"required"`
}

func (h *OwnerHandler) HandleSuspend(c *gin.Context) {
	var req suspendReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.uc.Suspend(c.Request.Context(), c.GetString("userEmail"), c.Param("id"), req.Reason); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *OwnerHandler) HandleReactivate(c *gin.Context) {
	if err := h.uc.Reactivate(c.Request.Context(), c.GetString("userEmail"), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type setStatusReq struct {
	Status string `json:"status" binding:"required"`
}

func (h *OwnerHandler) HandleSetStatus(c *gin.Context) {
	var req setStatusReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.uc.SetSubscriptionStatus(c.Request.Context(), c.GetString("userEmail"), c.Param("id"), req.Status); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type setCurrencyReq struct {
	Currency string `json:"currency" binding:"required" enums:"brl,usd"`
}

// HandleSetCurrency godoc
// @Summary      Correct account billing currency (owner)
// @Description  Owner-panel escape hatch for locale-based currency arbitrage: rewrites accounts.currency. Refused (409) when the account has an active Stripe subscription.
// @Tags         Owner
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path  string          true "Account ID"
// @Param        request body  setCurrencyReq  true "New currency"
// @Success      200  {object}  map[string]bool
// @Failure      400  {object}  domain.AppError "code: OWNER_INVALID_CURRENCY"
// @Failure      409  {object}  domain.AppError "code: OWNER_CURRENCY_HAS_SUBSCRIPTION — cancel the Stripe subscription first"
// @Router       /owner/accounts/{id}/currency [put]
func (h *OwnerHandler) HandleSetCurrency(c *gin.Context) {
	var req setCurrencyReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.uc.SetCurrency(c.Request.Context(), c.GetString("userEmail"), c.Param("id"), req.Currency); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type setSellerReq struct {
	// SellerEmail identifies the platform-account user to link. Empty
	// string (or omitted) unlinks the current seller.
	SellerEmail string `json:"seller_email"`
	// LeadSource is required when linking: 'seller_ref' | 'inbound'.
	LeadSource string `json:"lead_source"`
}

// HandleSetSeller links/reallocates/unlinks the seller (vendedores v3
// carteira) of a customer account. PUT /owner/accounts/:id/seller.
func (h *OwnerHandler) HandleSetSeller(c *gin.Context) {
	var req setSellerReq
	if !bindJSON(c, &req) {
		return
	}
	acc, err := h.uc.SetSeller(c.Request.Context(), c.GetString("userEmail"), c.Param("id"), req.SellerEmail, req.LeadSource)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, acc)
}

type createContractReq struct {
	CompanyName  string          `json:"company_name" binding:"required"`
	AdminEmail   string          `json:"admin_email" binding:"required"`
	AdminName    string          `json:"admin_name"`
	PlanID       string          `json:"plan_id" binding:"required"`
	TrialBypass  bool            `json:"trial_bypass"`
	BypassReason string          `json:"bypass_reason"`
	Overrides    setOverridesReq `json:"overrides"`
}

type applyContractReq struct {
	PlanID             string          `json:"plan_id" binding:"required"`
	TrialBypass        bool            `json:"trial_bypass"`
	BypassReason       string          `json:"bypass_reason"`
	PlatformKeyAllowed *bool           `json:"platform_key_allowed"`
	Overrides          setOverridesReq `json:"overrides"`
}

// HandleApplyContract converts an already-registered account onto a
// contract plan (plan + caps + trial-bypass + active status) in one
// call. The create-contract endpoint only works for a brand-new email;
// this is the path for existing accounts.
func (h *OwnerHandler) HandleApplyContract(c *gin.Context) {
	var req applyContractReq
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.uc.ApplyContract(c.Request.Context(), usecases.ApplyContractDTO{
		ActorEmail:         c.GetString("userEmail"),
		AccountID:          c.Param("id"),
		PlanID:             req.PlanID,
		TrialBypass:        req.TrialBypass,
		BypassReason:       req.BypassReason,
		PlatformKeyAllowed: req.PlatformKeyAllowed,
		Overrides: domain.AccountLimitOverrides{
			MaxAgents:                    req.Overrides.MaxAgents,
			MaxChannels:                  req.Overrides.MaxChannels,
			MaxSeats:                     req.Overrides.MaxSeats,
			MaxConversationsPerMonth:     req.Overrides.MaxConversationsPerMonth,
			MaxStorageMB:                 req.Overrides.MaxStorageMB,
			MaxAITokensPerMonth:          req.Overrides.MaxAITokensPerMonth,
			MaxBroadcastMessagesPerMonth: req.Overrides.MaxBroadcastMessagesPerMonth,
			MaxRecipientsPerCampaign:     req.Overrides.MaxRecipientsPerCampaign,
		},
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandleSystemHealth returns aggregate counts for the owner-panel
// system widget.
func (h *OwnerHandler) HandleSystemHealth(c *gin.Context) {
	out, err := h.uc.SystemHealth(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandleListInvoices returns the customer's last 10 Stripe invoices
// for the supplied account. Empty list when the account has no
// Stripe customer (contract plans / pre-checkout).
func (h *OwnerHandler) HandleListInvoices(c *gin.Context) {
	rows, err := h.uc.ListInvoices(c.Request.Context(), c.Param("id"), 10)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

// HandleUsageTrend returns the 30-day daily token + conversation
// rollup for a target account.
func (h *OwnerHandler) HandleUsageTrend(c *gin.Context) {
	rows, err := h.uc.UsageTrend(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

// HandleListConversations returns the latest conversations of a
// target account — read-only support tool.
func (h *OwnerHandler) HandleListConversations(c *gin.Context) {
	limit, offset := pagination(c)
	rows, err := h.uc.ListConversations(c.Request.Context(), c.Param("id"), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

// HandleTopAccounts returns the heaviest spenders this period.
func (h *OwnerHandler) HandleTopAccounts(c *gin.Context) {
	limit := 10
	if v := c.Query("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			limit = parsed
		}
	}
	rows, err := h.uc.TopAccounts(c.Request.Context(), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

type grantCreditsReq struct {
	Tokens int64 `json:"tokens" binding:"required"`
}

func (h *OwnerHandler) HandleGrantCredits(c *gin.Context) {
	var req grantCreditsReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.uc.GrantCredits(c.Request.Context(), c.GetString("userEmail"), c.Param("id"), req.Tokens); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type extendTrialReq struct {
	Days int `json:"days" binding:"required"`
}

func (h *OwnerHandler) HandleExtendTrial(c *gin.Context) {
	var req extendTrialReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.uc.ExtendTrial(c.Request.Context(), c.GetString("userEmail"), c.Param("id"), req.Days); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type impersonateReq struct {
	Minutes int `json:"minutes"`
}

func (h *OwnerHandler) HandleImpersonate(c *gin.Context) {
	var req impersonateReq
	_ = c.ShouldBindJSON(&req) // body optional; default to 15 inside the UC
	out, err := h.uc.Impersonate(c.Request.Context(), h.jwtSvc, c.GetString("userEmail"), c.Param("id"), req.Minutes)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *OwnerHandler) HandleCreateContract(c *gin.Context) {
	var req createContractReq
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.uc.CreateContract(c.Request.Context(), usecases.CreateContractDTO{
		ActorEmail:   c.GetString("userEmail"),
		CompanyName:  req.CompanyName,
		AdminEmail:   req.AdminEmail,
		AdminName:    req.AdminName,
		PlanID:       req.PlanID,
		TrialBypass:  req.TrialBypass,
		BypassReason: req.BypassReason,
		Overrides: domain.AccountLimitOverrides{
			MaxAgents:                    req.Overrides.MaxAgents,
			MaxChannels:                  req.Overrides.MaxChannels,
			MaxSeats:                     req.Overrides.MaxSeats,
			MaxConversationsPerMonth:     req.Overrides.MaxConversationsPerMonth,
			MaxStorageMB:                 req.Overrides.MaxStorageMB,
			MaxAITokensPerMonth:          req.Overrides.MaxAITokensPerMonth,
			MaxBroadcastMessagesPerMonth: req.Overrides.MaxBroadcastMessagesPerMonth,
			MaxRecipientsPerCampaign:     req.Overrides.MaxRecipientsPerCampaign,
		},
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}
