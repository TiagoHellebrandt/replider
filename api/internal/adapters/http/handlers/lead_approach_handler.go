package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// HandleDraftApproach serves POST /v1/leads/:id/draft — generates an
// AI-suggested approach and queues the lead for human approval
// (status=pending_approval). It sends NOTHING (US-020).
// @Summary     Generate an approach draft
// @Description Asks the account's AI provider for a suggested first message and queues the lead for human approval. It sends NOTHING.
// @Tags        Leads
// @Produce     json
// @Param       id path string true "Lead ID"
// @Success     200 {object} domain.Lead
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/{id}/draft [post]
func (h *LeadHandler) HandleDraftApproach(c *gin.Context) {
	accountID := getAccountID(c)
	if !h.ensureLeadMutable(c) {
		return
	}
	lead, err := h.approachUC.Draft(c.Request.Context(), accountID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, lead)
}

type saveDraftReq struct {
	// Text is what the person wrote. Empty is allowed and meaningful: clearing
	// the box is an edit, and refusing to persist it would resurrect a draft
	// they deliberately deleted.
	Text string `json:"text"`
}

// HandleSaveDraft serves PATCH /v1/leads/:id/approach — persists a
// human-written approach WITHOUT sending it and without queuing the lead.
//
// The panel had no way to keep typed text: the only writers of approach_draft
// were the AI generator and the send. Closing the panel threw the words away.
// @Summary     Store a human-written approach
// @Description Persists the text without sending it and without queuing the lead: queuing is a decision ("this is ready for review"), and storing half a sentence is not that decision. An empty body clears the draft. Refused once the lead has been contacted, because that draft is now a record of what was sent.
// @Tags        Leads
// @Accept      json
// @Produce     json
// @Param       id      path string       true "Lead ID"
// @Param       payload body saveDraftReq  true "The approach text"
// @Success     200 {object} domain.Lead
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/{id}/approach [patch]
func (h *LeadHandler) HandleSaveDraft(c *gin.Context) {
	if !h.ensureLeadMutable(c) {
		return
	}
	var req saveDraftReq
	if !bindJSON(c, &req) {
		return
	}
	lead, err := h.approachUC.SaveDraft(
		c.Request.Context(), getAccountID(c), c.Param("id"), req.Text)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, lead)
}

// approveSendButton is one declared button of the chosen approved template,
// forwarded so the anti-ban opt-out guard sees the opt-out quick-reply.
type approveSendButton struct {
	SubType string `json:"sub_type"`
	Text    string `json:"text"`
	Payload string `json:"payload"`
}

type approveSendReq struct {
	// Text is the FINAL, human-reviewed/edited message — delivered as the
	// approved template's BODY parameter.
	Text string `json:"text" binding:"required"`
	// TemplateName is the approved WhatsApp template the approach is sent
	// through. Cold first-contact on WhatsApp is only deliverable via a
	// template (US-020) — but that rule is WhatsApp's, so it is enforced by
	// the use case AFTER the channel is known, not by a binding tag here.
	// A binding:"required" made every EMAIL approval fail at the door.
	TemplateName     string              `json:"template_name"`
	TemplateLanguage string              `json:"template_language"`
	TemplateCategory string              `json:"template_category"`
	Buttons          []approveSendButton `json:"template_buttons"`

	// Email path (US-M009).
	Subject string `json:"subject"`
	// EmailTemplateID/Version attribute the send to a stored template so the
	// reply it earns lands on the right numbers. Optional — a text written
	// from scratch has no template.
	EmailTemplateID      string `json:"email_template_id"`
	EmailTemplateVersion int    `json:"email_template_version"`
	// ViaBulk routes the send through the platform's bulk provider (hybrid
	// model): batch approvals set it, one-by-one approvals never do. Refused
	// with BULK_SENDING_NOT_READY unless the domain is bulk-verified.
	ViaBulk bool `json:"via_bulk"`
}

// HandleApproveSend serves POST /v1/leads/:id/approve-send — sends the
// human-edited approach via the chosen approved template, enforcing plan limits
// and the opt-out block. Only fires for a lead awaiting approval (US-020).
// @Summary     Approve and send an approach
// @Description Sends the HUMAN-EDITED text. There is deliberately no route that sends an AI draft unreviewed — that gate is the product's whole posture on cold outbound. Only fires for a lead awaiting approval.
// @Tags        Leads
// @Accept      json
// @Produce     json
// @Param       id      path string         true "Lead ID"
// @Param       payload body approveSendReq  true "Final text, plus the template or subject the channel needs"
// @Success     200 {object} domain.Lead
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/{id}/approve-send [post]
func (h *LeadHandler) HandleApproveSend(c *gin.Context) {
	accountID := getAccountID(c)
	// The heaviest write in the product: this one reaches a real customer, so the
	// permission is settled before the payload is even read.
	if !h.ensureLeadMutable(c) {
		return
	}
	var req approveSendReq
	if !bindJSON(c, &req) {
		return
	}
	in := usecases.ApproachSendInput{
		Text:             req.Text,
		ActorUserID:      getUserID(c),
		TemplateName:     req.TemplateName,
		TemplateLanguage: req.TemplateLanguage,
		TemplateCategory: req.TemplateCategory,

		Subject:              req.Subject,
		EmailTemplateID:      req.EmailTemplateID,
		EmailTemplateVersion: req.EmailTemplateVersion,
		ViaBulk:              req.ViaBulk,
	}
	for _, b := range req.Buttons {
		in.Buttons = append(in.Buttons, usecases.ApproachTemplateButton{
			SubType: b.SubType, Text: b.Text, Payload: b.Payload,
		})
	}
	lead, err := h.approachUC.ApproveSend(c.Request.Context(), accountID, c.Param("id"), in)
	if err != nil {
		// The daily cap stops being a wall: the approval is a decision
		// already made, so it becomes a scheduled send for the next morning
		// the budget allows. Any scheduling error falls back to the original
		// refusal — the person can still come back tomorrow.
		if appErr, ok := err.(*domain.AppError); ok &&
			appErr.Code == domain.ErrEmailDailyCap.Code &&
			h.approachUC.CanScheduleApproach() {
			sendAt, schedErr := h.approachUC.ScheduleApproach(
				c.Request.Context(), accountID, c.Param("id"), in)
			if schedErr == nil {
				c.JSON(http.StatusOK, gin.H{
					"scheduled":     true,
					"scheduled_for": sendAt,
				})
				return
			}
		}
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, lead)
}

// HandleDraftBatch serves POST /v1/funnels/:id/leads/draft-batch — bulk-queues
// drafts for a funnel's not-yet-approached leads, filtering opted-out contacts
// and reporting how many were skipped (US-020). Sends nothing.
type reviewApproachRequest struct {
	Text string `json:"text"`
}

// HandleReviewApproach serves POST /v1/leads/:id/review-approach.
//
// Takes the text about to be SENT, not the draft that was generated: the
// person editing can introduce a promise the model never wrote, and a
// draft-time check would have passed it.
//
// Never blocks. It returns what it found and the human decides — moving that
// call to a model would take it away from the person who answers for the
// message.
// @Summary     Second opinion on the text about to be sent
// @Description Reads the text as it stands, not the draft that was generated: the person editing can introduce a promise the model never wrote. Never blocks — it reports what it found and the human decides.
// @Tags        Leads
// @Accept      json
// @Produce     json
// @Param       id      path string                true "Lead ID"
// @Param       payload body reviewApproachRequest   true "The text to review"
// @Success     200 {object} usecases.ApproachReview
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     503 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/{id}/review-approach [post]
func (h *LeadHandler) HandleReviewApproach(c *gin.Context) {
	if h.reviewUC == nil {
		respondError(c, &domain.AppError{
			Code: "APPROACH_REVIEW_UNAVAILABLE", Message: "review is not configured"})
		return
	}
	var req reviewApproachRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.reviewUC.Review(c.Request.Context(), actorFrom(c), c.Param("id"), req.Text)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// @Summary     Bulk-queue approach drafts for a funnel
// @Description Fills the approval queue for a funnel's not-yet-approached leads, skipping opted-out contacts. Sends nothing. Reports what the plan's allowance did not cover rather than silently returning fewer.
// @Tags        Leads
// @Produce     json
// @Param       id    path  string true  "Funnel ID"
// @Param       limit query int    false "Cap on how many leads to draft for"
// @Success     200 {object} usecases.BatchDraftResult
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /funnels/{id}/leads/draft-batch [post]
func (h *LeadHandler) HandleDraftBatch(c *gin.Context) {
	accountID := getAccountID(c)
	limit := 0
	if v := c.Query("limit"); v != "" {
		limit, _ = strconv.Atoi(v)
	}
	res, err := h.approachUC.DraftBatch(c.Request.Context(), accountID, c.Param("id"), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// batchLeadIDsRequest carries the ids for either queue batch action.
type batchLeadIDsRequest struct {
	LeadIDs []string `json:"lead_ids" binding:"required"`
}

// HandleBatchDiscard serves POST /v1/leads/batch-discard — sweeping a group
// of queued drafts (typically the pre-catalogue generic ones) in one call.
// @Summary     Discard queued drafts in batch
// @Description Moves every listed lead still waiting for approval to lost. Leads the caller may not mutate, or that are not in the queue, are counted as skipped rather than failing the batch.
// @Tags        Leads
// @Accept      json
// @Produce     json
// @Param       payload body batchLeadIDsRequest true "Lead ids (max 200)"
// @Success     200 {object} usecases.BatchDiscardResult
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/batch-discard [post]
func (h *LeadHandler) HandleBatchDiscard(c *gin.Context) {
	var req batchLeadIDsRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.leadUC.DiscardBatch(c.Request.Context(), actorFrom(c), req.LeadIDs)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// HandleBatchRedraft serves POST /v1/leads/batch-redraft — regenerating the
// AI draft for queued leads after the catalogue was imported.
// @Summary     Regenerate queued drafts in batch
// @Description Redrafts up to 8 leads per call (each is a live AI call); the response lists the remainder for the client to resubmit. Stops early on an account-wide refusal (spend cap, empty catalogue) and names it in stopped_reason.
// @Tags        Leads
// @Accept      json
// @Produce     json
// @Param       payload body batchLeadIDsRequest true "Lead ids (max 200)"
// @Success     200 {object} usecases.BatchRedraftResult
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/batch-redraft [post]
func (h *LeadHandler) HandleBatchRedraft(c *gin.Context) {
	var req batchLeadIDsRequest
	if !bindJSON(c, &req) {
		return
	}
	// Ownership is filtered HERE, per lead, before any AI money is spent:
	// the approach use case scopes by account only, and a vendedor must not
	// be able to redraft (and re-bill) a colleague's queue.
	actor := actorFrom(c)
	authorized := make([]string, 0, len(req.LeadIDs))
	skipped := []string{}
	for _, id := range req.LeadIDs {
		if _, err := h.leadUC.MutableForActor(c.Request.Context(), actor, id); err != nil {
			skipped = append(skipped, id)
			continue
		}
		authorized = append(authorized, id)
	}
	res, err := h.approachUC.RedraftBatch(c.Request.Context(), actor.AccountID, authorized)
	if err != nil {
		respondError(c, err)
		return
	}
	res.FailedLeadIDs = append(res.FailedLeadIDs, skipped...)
	c.JSON(http.StatusOK, res)
}
