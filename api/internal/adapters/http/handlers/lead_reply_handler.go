package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// SetReplyUC wires the lead-scoped reply (optional-setter, like the timeline:
// nil answers 503 rather than silently accepting text nobody will send).
func (h *LeadHandler) SetReplyUC(uc *usecases.LeadReplyUseCase) { h.replyUC = uc }

// ErrLeadReplyUnavailable is the 503 for an installation where the reply path
// was not wired.
var errLeadReplyUnavailable = &domain.AppError{Code: "LEAD_REPLY_UNAVAILABLE", Message: "lead reply is not available"}

type leadReplyRequest struct {
	Text    string `json:"text"`
	Subject string `json:"subject"`
}

// HandleReply answers the lead's customer on the thread they already share.
//
// Authorised by the LEAD's rules (MutableForActor), not the inbox's: a
// vendedor cannot open /conversations/*, but answering the reply to her own
// proposal is exactly her job.
//
//	@Summary		Reply to a lead's customer
//	@Description	Sends a human message on the conversation behind this lead's contact. The bot is paused on that thread first. 409 LEAD_NO_CONVERSATION when nobody has exchanged a message with the lead yet.
//	@Tags			leads
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Lead ID"
//	@Param			body	body		leadReplyRequest	true	"text (required), subject (e-mail only)"
//	@Success		201		{object}	domain.Message
//	@Failure		400		{object}	domain.AppError
//	@Failure		403		{object}	domain.AppError
//	@Failure		404		{object}	domain.AppError
//	@Failure		409		{object}	domain.AppError
//	@Failure		503		{object}	domain.AppError
//	@Security		BearerAuth
//	@Router			/leads/{id}/reply [post]
func (h *LeadHandler) HandleReply(c *gin.Context) {
	if h.replyUC == nil {
		respondError(c, errLeadReplyUnavailable)
		return
	}
	// Permission before payload, same order as every other lead write.
	if !h.ensureLeadMutable(c) {
		return
	}
	var req leadReplyRequest
	if !bindJSON(c, &req) {
		return
	}
	msg, err := h.replyUC.Execute(c.Request.Context(), actorFrom(c), usecases.LeadReplyDTO{
		LeadID:   c.Param("id"),
		Text:     req.Text,
		Subject:  req.Subject,
		UserName: c.GetString("userName"),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, msg)
}
