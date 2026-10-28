package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// The door for customers she already has.
//
// Its own file rather than another block in lead_handler.go, which is already
// past the 400-line guideline.

type contactsToFunnelRequest struct {
	ContactIDs []string `json:"contact_ids"`
	// OwnerUserID assigns the created leads to a specific salesperson. Omitted
	// means unassigned, which stays visible to the whole team — NOT the caller,
	// because an admin importing the company's base is not claiming it.
	OwnerUserID string `json:"owner_user_id"`
}

// HandleContactsToFunnel serves POST /v1/funnels/:id/leads/from-contacts.
//
// @Summary     Send existing contacts to a funnel
// @Description Puts customers already in the CRM into a sales funnel, in bulk. Creates no contact and rewrites no name — the only new fact is which funnel they belong to. Idempotent per (funnel, contact): somebody already in the funnel is reported as skipped and their pipeline stage is left untouched. One bad row never aborts the batch; the response names every contact that failed.
// @Tags        Leads
// @Accept      json
// @Produce     json
// @Param       id      path string                  true "Funnel ID"
// @Param       payload body contactsToFunnelRequest true "Contacts to send"
// @Success     200 {object} usecases.ContactsToFunnelResult
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     403 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /funnels/{id}/leads/from-contacts [post]
func (h *LeadHandler) HandleContactsToFunnel(c *gin.Context) {
	var req contactsToFunnelRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.leadUC.CreateFromContacts(c.Request.Context(), usecases.ContactsToFunnelInput{
		AccountID:   getAccountID(c),
		FunnelID:    c.Param("id"),
		ContactIDs:  req.ContactIDs,
		OwnerUserID: req.OwnerUserID,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	// 200 rather than 201 even when leads were created: a partial batch is the
	// normal outcome here, and "created" is a count in the body, not a status.
	c.JSON(http.StatusOK, res)
}

// Referenced so swag resolves domain.AppError in the annotations above.
var _ = domain.AppError{}
