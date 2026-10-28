package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Account invites — the owner side of the sales-led signup flow
// (docs/fluxo-criacao-conta.md). Own file, same reason as every other
// OwnerHandler extension: owner_handler.go is past the size guideline.

// SetInviteUC wires the invite emission. Optional setter; unwired, the
// routes answer 503.
func (h *OwnerHandler) SetInviteUC(uc *usecases.InviteUseCase) { h.inviteUC = uc }

var errInvitesUnavailable = &domain.AppError{
	Code:    "INVITES_UNAVAILABLE",
	Message: "convites não configurados neste ambiente",
}

type createInviteReq struct {
	Email  string `json:"email" binding:"required"`
	PlanID string `json:"plan_id" binding:"required"`
	// Overrides uses the SAME shape as apply-contract, so the owner screen
	// can share the form.
	Overrides     domain.AccountLimitOverrides `json:"overrides"`
	ExpiresInDays int                          `json:"expires_in_days"`
}

// HandleCreateInvite serves POST /owner/invites.
//
// The response is the ONE exposure of the invite token — it is stored only
// as a hash. Losing it means revoke + reissue, never recover.
func (h *OwnerHandler) HandleCreateInvite(c *gin.Context) {
	if h.inviteUC == nil {
		respondError(c, errInvitesUnavailable)
		return
	}
	var req createInviteReq
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.inviteUC.Create(c.Request.Context(), usecases.CreateInviteDTO{
		ActorUserID:   c.GetString("userID"),
		ActorEmail:    c.GetString("userEmail"),
		Email:         req.Email,
		PlanID:        req.PlanID,
		Overrides:     req.Overrides,
		ExpiresInDays: req.ExpiresInDays,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

// HandleListInvites serves GET /owner/invites — pending only, never hashes.
func (h *OwnerHandler) HandleListInvites(c *gin.Context) {
	if h.inviteUC == nil {
		respondError(c, errInvitesUnavailable)
		return
	}
	invites, err := h.inviteUC.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"invites": invites})
}

// HandleRevokeInvite serves POST /owner/invites/:id/revoke.
func (h *OwnerHandler) HandleRevokeInvite(c *gin.Context) {
	if h.inviteUC == nil {
		respondError(c, errInvitesUnavailable)
		return
	}
	if err := h.inviteUC.Revoke(c.Request.Context(), c.GetString("userEmail"), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleListAllPlans serves GET /owner/plans — the UNFILTERED catalogue.
//
// The public /plans hides unlisted plans on purpose, which includes every
// SDR contract tier — exactly the ones an invite (or apply-contract) needs
// to point at. The owner panel needs the whole table.
func (h *OwnerHandler) HandleListAllPlans(c *gin.Context) {
	if h.inviteUC == nil {
		respondError(c, errInvitesUnavailable)
		return
	}
	plans, err := h.inviteUC.Plans(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, plans)
}
