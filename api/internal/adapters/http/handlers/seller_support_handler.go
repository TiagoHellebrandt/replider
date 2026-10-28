package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Seller support view + account note (canal-interno US-007). Methods
// live on SellerHandler; the use case is attached via the optional
// setter so NewSellerHandler and its test call sites stay untouched.

// SetSupportUC wires the grouped-threads / account-note use case
// (optional; nil → 503 on the endpoints).
func (h *SellerHandler) SetSupportUC(uc *usecases.SellerSupportUseCase) {
	h.supportUC = uc
}

// SetSupportThreadUC wires the outbound-first thread opener (US-008) —
// the same use case MeHandler drives for the customer side, reused for
// the co-pilot's 1-click action (optional; nil → 503).
func (h *SellerHandler) SetSupportThreadUC(uc *usecases.SupportThreadUseCase) {
	h.supportThreadUC = uc
}

var errSellerSupportUnavailable = &domain.AppError{Code: "SUPPORT_NOT_AVAILABLE", Message: "seller support view is not available"}

// HandleGetSupportThreads godoc
// @Summary      My carteira's internal CS threads, grouped by account (seller panel)
// @Description  Lists every internal support thread belonging to the calling seller's carteira accounts, grouped account → user threads (name, role, last message, unread = customer messages awaiting a platform reply), with the account-level shared note (US-007). Platform-account only; 404 elsewhere.
// @Tags         Seller
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  usecases.SellerSupportView
// @Failure      401  {object}  domain.AppError
// @Failure      404  {object}  domain.AppError  "outside the platform account"
// @Router       /seller/support-threads [get]
func (h *SellerHandler) HandleGetSupportThreads(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	if h.supportUC == nil {
		respondError(c, errSellerSupportUnavailable)
		return
	}
	out, err := h.supportUC.ListGrouped(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandleGetAccountNote godoc
// @Summary      Shared internal note of a thread's customer account
// @Description  Resolves the internal thread's customer account and returns the account-level shared note with its visible audit metadata — who edited, when (US-007). Empty note when never written. Sellers and platform admins only. Platform-account only; 404 elsewhere.
// @Tags         Seller
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "internal-thread conversation id"
// @Success      200  {object}  domain.AccountInternalNote
// @Failure      401  {object}  domain.AppError
// @Failure      403  {object}  domain.AppError  "caller is neither a seller nor a platform admin"
// @Failure      404  {object}  domain.AppError  "not an internal thread; or outside the platform account"
// @Router       /seller/conversations/{id}/account-note [get]
func (h *SellerHandler) HandleGetAccountNote(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	if h.supportUC == nil {
		respondError(c, errSellerSupportUnavailable)
		return
	}
	out, err := h.supportUC.GetNote(c.Request.Context(), userID, c.GetString("role"), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type accountNoteRequest struct {
	Note string `json:"note"`
}

// HandleSetAccountNote godoc
// @Summary      Edit the shared internal note of a thread's customer account
// @Description  Upserts the account-level shared note (one per customer account, last-writer-wins) stamping who/when, and writes an audit_logs row (US-007). An empty value clears it. Sellers and platform admins only. Platform-account only; 404 elsewhere.
// @Tags         Seller
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string              true  "internal-thread conversation id"
// @Param        request  body  accountNoteRequest  true  "note text (empty clears; max 4000 chars)"
// @Success      200  {object}  domain.AccountInternalNote
// @Failure      400  {object}  domain.AppError  "note too long"
// @Failure      401  {object}  domain.AppError
// @Failure      403  {object}  domain.AppError  "caller is neither a seller nor a platform admin"
// @Failure      404  {object}  domain.AppError  "not an internal thread; or outside the platform account"
// @Router       /seller/conversations/{id}/account-note [put]
func (h *SellerHandler) HandleSetAccountNote(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	if h.supportUC == nil {
		respondError(c, errSellerSupportUnavailable)
		return
	}
	var req accountNoteRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.supportUC.SetNote(c.Request.Context(), userID, c.GetString("role"), c.Param("id"), req.Note)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type openSupportThreadRequest struct {
	AccountID string `json:"account_id" binding:"required"`
}

// HandleOpenSupportThread godoc
// @Summary      Open (or reuse) the internal CS thread with a carteira account's admin
// @Description  Outbound-first thread creation for the co-pilot's 1-click action (US-008): resolves the target carteira account's admin user and returns their internal thread, creating it when the customer never wrote. A fresh thread is born in takeover mode assigned to the calling seller, so the composer send passes the bot-pause gate with no extra step. Only the account's own carteira seller may open it. Platform-account only; 404 elsewhere.
// @Tags         Seller
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  openSupportThreadRequest  true  "target customer account"
// @Success      200  {object}  usecases.SupportOpenThreadResult
// @Failure      400  {object}  domain.AppError  "account has no active admin"
// @Failure      401  {object}  domain.AppError
// @Failure      404  {object}  domain.AppError  "account not in caller's carteira; or outside the platform account"
// @Failure      503  {object}  domain.AppError  "internal channel not configured"
// @Router       /seller/support-threads/open [post]
func (h *SellerHandler) HandleOpenSupportThread(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	if h.supportThreadUC == nil {
		respondError(c, errSellerSupportUnavailable)
		return
	}
	var req openSupportThreadRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.supportThreadUC.OpenThread(c.Request.Context(), userID, req.AccountID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}
