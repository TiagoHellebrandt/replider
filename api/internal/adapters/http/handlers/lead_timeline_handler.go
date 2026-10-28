package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// SetTimelineUseCase wires the lead's conversation history.
//
// Optional-setter so `NewLeadHandler` and every one of its call sites stay as
// they are; nil answers 503 rather than an empty list, because "nobody has
// written to this customer" and "this is not working" are different statements.
func (h *LeadHandler) SetTimelineUseCase(uc *usecases.LeadTimelineUseCase) {
	h.timelineUC = uc
}

// HandleTimeline returns what was exchanged with this lead's contact.
//
// Authorised by the LEAD's rules, not the inbox's. A vendedor does not satisfy
// `attendant` and never will — reading every conversation in the account is not
// their job — but reading the replies to their own lead is exactly their job,
// and until now they sent proposals and could not see the answers.
//
//	@Summary		Conversation history for a lead
//	@Description	The recent messages exchanged with this lead's contact, oldest first. Authorised by lead ownership, not by inbox role.
//	@Tags			leads
//	@Produce		json
//	@Param			id	path		string	true	"Lead ID"
//	@Success		200	{object}	usecases.LeadTimeline
//	@Failure		403	{object}	domain.AppError
//	@Failure		404	{object}	domain.AppError
//	@Failure		503	{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/leads/{id}/timeline [get]
func (h *LeadHandler) HandleTimeline(c *gin.Context) {
	if h.timelineUC == nil {
		respondError(c, domain.ErrLeadTimelineUnavailable)
		return
	}

	timeline, err := h.timelineUC.Execute(
		c.Request.Context(), actorFrom(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, timeline)
}
