package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// SellerHandler serves the seller (vendedores) panel endpoints. Every
// route is mounted behind middleware.RequirePlatformAccount, so only
// authenticated users of the Replider platform account reach it — the
// routes 404 everywhere else.
type SellerHandler struct {
	ensureRefCodeUC *usecases.EnsureSellerRefCodeUseCase
	convertLeadUC   *usecases.ConvertLeadUseCase
	carteiraUC      *usecases.SellerCarteiraUseCase
	statementUC     *usecases.SellerStatementUseCase
	slaUC           *usecases.SellerSLAUseCase
	suggestionsUC   *usecases.SellerSuggestionsUseCase
	// supportUC (canal-interno US-007) is attached via SetSupportUC —
	// optional-setter pattern so the constructor stays untouched.
	supportUC *usecases.SellerSupportUseCase
	// supportThreadUC (canal-interno US-008) is attached via
	// SetSupportThreadUC: the co-pilot's outbound-first thread opener.
	supportThreadUC *usecases.SupportThreadUseCase
}

func NewSellerHandler(ensureRefCodeUC *usecases.EnsureSellerRefCodeUseCase, convertLeadUC *usecases.ConvertLeadUseCase, carteiraUC *usecases.SellerCarteiraUseCase, statementUC *usecases.SellerStatementUseCase, slaUC *usecases.SellerSLAUseCase, suggestionsUC *usecases.SellerSuggestionsUseCase) *SellerHandler {
	return &SellerHandler{ensureRefCodeUC: ensureRefCodeUC, convertLeadUC: convertLeadUC, carteiraUC: carteiraUC, statementUC: statementUC, slaUC: slaUC, suggestionsUC: suggestionsUC}
}

// HandleGetSuggestions godoc
// @Summary      My co-pilot suggestion feed (seller panel)
// @Description  Computes proactive per-account suggestions for the calling seller's carteira from signals the platform already collects — takeover rate, conversation pace, plan-allowance usage, onboarding checklist, WhatsApp ban risk and NPS (US-014). On-demand with a short cache; suggestions dismissed/done this cycle are filtered out. Platform-account only; 404 elsewhere.
// @Tags         Seller
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  usecases.SellerSuggestionsFeed
// @Failure      401  {object}  domain.AppError
// @Failure      404  {object}  domain.AppError  "outside the platform account"
// @Router       /seller/suggestions [get]
func (h *SellerHandler) HandleGetSuggestions(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	out, err := h.suggestionsUC.Execute(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type suggestionStateRequest struct {
	AccountID      string `json:"account_id" binding:"required"`
	SuggestionType string `json:"suggestion_type" binding:"required"`
	Status         string `json:"status" binding:"required"`
	// ChannelID scopes channel-level suggestions (whatsapp_ban_risk is
	// dismissed per channel, US-022). Ignored for account-level types.
	ChannelID string `json:"channel_id"`
}

// HandleSetSuggestionState godoc
// @Summary      Dismiss or complete a co-pilot suggestion
// @Description  Persists dismissed/done for one suggestion type on one carteira account in the current cycle, so it stops appearing in the feed until next month (US-014). Only the calling seller's own carteira accounts are accepted. Platform-account only; 404 elsewhere.
// @Tags         Seller
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  suggestionStateRequest  true  "suggestion key + status (dismissed|done)"
// @Success      204
// @Failure      400  {object}  domain.AppError  "unknown suggestion type or status"
// @Failure      401  {object}  domain.AppError
// @Failure      404  {object}  domain.AppError  "account not in carteira; or outside the platform account"
// @Router       /seller/suggestions/state [post]
func (h *SellerHandler) HandleSetSuggestionState(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	var req suggestionStateRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.suggestionsUC.SetState(c.Request.Context(), userID, req.AccountID, req.SuggestionType, req.ChannelID, req.Status); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleGetSLA godoc
// @Summary      My live SLA status (seller panel)
// @Description  Measures the calling seller's CURRENT cycle first-response SLA on demand — % within the threshold, the ladder step the streak would land on (none/warning/reduced/flag_reallocation), the previous cycle's settled step, the resolved SLA settings (so the rule is visible) and the list of breached conversations. Read-only preview: settled states are written by the payroll compute (US-012). Platform-account only; 404 elsewhere.
// @Tags         Seller
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  usecases.SellerSLAStatus
// @Failure      401  {object}  domain.AppError
// @Failure      404  {object}  domain.AppError  "outside the platform account"
// @Router       /seller/sla [get]
func (h *SellerHandler) HandleGetSLA(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	out, err := h.slaUC.Execute(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandleGetStatement godoc
// @Summary      My commission statement (seller panel)
// @Description  Returns the calling seller's full ledger for one cycle with the per-account profit breakdown — revenue, Stripe fee, AI cost, tax, infra, net profit, rate applied (with reason) and commission — plus cycle totals, the cycle selector list and the seller's PIX key. An open (current) cycle is flagged is_open: a partial preview, not a settled statement (US-011). Platform-account only; 404 elsewhere.
// @Tags         Seller
// @Produce      json
// @Security     BearerAuth
// @Param        cycle  query  string  false  "cycle YYYY-MM (default: last closed month)"
// @Success      200  {object}  usecases.SellerStatement
// @Failure      400  {object}  domain.AppError  "invalid cycle"
// @Failure      401  {object}  domain.AppError
// @Failure      404  {object}  domain.AppError  "outside the platform account"
// @Router       /seller/commissions [get]
func (h *SellerHandler) HandleGetStatement(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	out, err := h.statementUC.Execute(c.Request.Context(), userID, c.Query("cycle"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type updatePixKeyRequest struct {
	PixKey string `json:"pix_key"`
}

// HandleUpdatePixKey godoc
// @Summary      Set my PIX payout key
// @Description  Stores the calling seller's own PIX key for the manual payout flow (US-011/US-013). An empty value clears it. Platform-account only; 404 elsewhere.
// @Tags         Seller
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  updatePixKeyRequest  true  "PIX key (empty clears)"
// @Success      204
// @Failure      400  {object}  domain.AppError  "key too long"
// @Failure      401  {object}  domain.AppError
// @Failure      404  {object}  domain.AppError  "outside the platform account"
// @Router       /seller/pix-key [put]
func (h *SellerHandler) HandleUpdatePixKey(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	var req updatePixKeyRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.statementUC.UpdatePixKey(c.Request.Context(), userID, req.PixKey); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleGetCarteira godoc
// @Summary      My carteira (seller panel)
// @Description  Returns the calling seller's personal ?ref= link, header totals (accounts, pending commission) and each linked account with plan/MRR, attribution track and the last closed cycle's net profit + commission (US-010). Platform-account only; 404 elsewhere.
// @Tags         Seller
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  usecases.SellerCarteira
// @Failure      401  {object}  domain.AppError
// @Failure      404  {object}  domain.AppError  "outside the platform account"
// @Router       /seller/carteira [get]
func (h *SellerHandler) HandleGetCarteira(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	out, err := h.carteiraUC.Execute(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandleEnsureRefCode godoc
// @Summary      Get or create my seller prospecting code
// @Description  Returns the calling seller's personal ?ref= code and shareable signup link, minting the code on first call (idempotent — always the same code afterwards). Platform-account only; 404 elsewhere (US-003).
// @Tags         Seller
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  usecases.SellerRefInfo
// @Failure      401  {object}  domain.AppError
// @Failure      404  {object}  domain.AppError  "outside the platform account"
// @Router       /seller/ref-code [post]
func (h *SellerHandler) HandleEnsureRefCode(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	info, err := h.ensureRefCodeUC.Execute(c.Request.Context(), userID)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

// HandleSearchLeadAccounts godoc
// @Summary      Search customer accounts for lead conversion
// @Description  Finds customer accounts by name or user email for the "convert lead" dialog (US-004). Queries under 2 characters return an empty list. The platform account itself is never returned. Max 10 results, newest first.
// @Tags         Seller
// @Produce      json
// @Security     BearerAuth
// @Param        search  query  string  true  "name or email fragment (min 2 chars)"
// @Success      200  {array}   domain.SellerLeadAccount
// @Failure      401  {object}  domain.AppError
// @Failure      404  {object}  domain.AppError  "outside the platform account"
// @Router       /seller/lead-accounts [get]
func (h *SellerHandler) HandleSearchLeadAccounts(c *gin.Context) {
	out, err := h.convertLeadUC.Search(c.Request.Context(), c.Query("search"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type convertLeadRequest struct {
	AccountID string `json:"account_id" binding:"required"`
}

// HandleConvertLead godoc
// @Summary      Convert an inbound lead conversation into a customer account
// @Description  Stamps the target account with the conversation's ASSIGNED attendant as seller (lead_source='inbound', 20% track). Only the assigned attendant or an admin may convert; an account already linked to a seller is a 409 and is never overwritten (US-004).
// @Tags         Seller
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string              true  "conversation id (platform account)"
// @Param        request  body  convertLeadRequest  true  "target customer account"
// @Success      200  {object}  domain.SellerLeadAccount
// @Failure      400  {object}  domain.AppError  "conversation unassigned / platform account target"
// @Failure      401  {object}  domain.AppError
// @Failure      403  {object}  domain.AppError  "caller is neither the assignee nor an admin"
// @Failure      404  {object}  domain.AppError  "conversation or account not found; or outside the platform account"
// @Failure      409  {object}  domain.AppError  "account already linked to a seller"
// @Router       /seller/conversations/{id}/convert [post]
func (h *SellerHandler) HandleConvertLead(c *gin.Context) {
	var req convertLeadRequest
	if !bindJSON(c, &req) {
		return
	}
	acc, err := h.convertLeadUC.Execute(
		c.Request.Context(),
		getUserID(c),
		c.GetString("role"),
		c.Param("id"),
		req.AccountID,
	)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, acc)
}
