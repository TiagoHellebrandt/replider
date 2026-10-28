package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// HandleListMessages godoc
// @Summary      List messages for a conversation
// @Description  Returns messages for a conversation, ordered by created_at ASC.
//
//	GCS media_url fields are resolved to media-proxy URLs on each request.
//
// @Tags         conversations
// @Produce      json
// @Param        id     path  string true  "Conversation ID"
// @Param        limit  query int    false "Maximum number of results (1-200)" default(100)
// @Param        offset query int    false "Number of results to skip" default(0)
// @Success      200  {array}   domain.Message
// @Failure      404  {object}  domain.AppError
// @Security     BearerAuth
// @Router       /conversations/{id}/messages [get]
func (h *ConversationHandler) HandleListMessages(c *gin.Context) {
	convID := c.Param("id")
	acctID := getAccountID(c)
	if acctID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "AUTH_UNAUTHORIZED", Message: "unauthorized"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 200 {
		limit = 200
	}
	if limit < 1 {
		limit = 1
	}
	if offset < 0 {
		offset = 0
	}

	// Verify conversation ownership
	conv, err := h.convRepo.GetByID(c.Request.Context(), convID, acctID)
	if err != nil || conv == nil {
		c.JSON(http.StatusNotFound, domain.ErrConversationNotFound)
		return
	}

	// Load-more (older than a cursor). `before` = RFC3339 created_at of the
	// oldest message the client holds; the page is the `limit` messages
	// before it, chronological. A cursor rather than an offset because new
	// messages keep landing while the attendant scrolls up.
	var messages []*domain.Message
	if before := c.Query("before"); before != "" {
		var cursor time.Time
		if before == "latest" {
			// The newest page, on the SERVER's clock — a client-side "now"
			// would drop the newest messages on a device whose clock lags.
			cursor = time.Now().Add(time.Minute)
		} else {
			parsed, parseErr := time.Parse(time.RFC3339Nano, before)
			if parseErr != nil {
				respondError(c, &domain.AppError{Code: "VALIDATION_ERROR", Message: "before must be an RFC3339 timestamp or 'latest'"})
				return
			}
			cursor = parsed
		}
		pager, ok := h.msgRepo.(interface {
			ListByConversationBefore(ctx context.Context, conversationID string, before time.Time, limit int32) ([]*domain.Message, error)
		})
		if !ok {
			c.JSON(http.StatusServiceUnavailable, domain.AppError{Code: "MESSAGES_BEFORE_UNAVAILABLE", Message: "message paging not configured"})
			return
		}
		messages, err = pager.ListByConversationBefore(c.Request.Context(), convID, cursor, int32(limit)) //nolint:gosec // G115: clamped to [1,200] above
	} else {
		messages, err = h.msgRepo.ListByConversationID(c.Request.Context(), convID, int32(limit), int32(offset)) //nolint:gosec // G115: clamped above
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to list messages"})
		return
	}

	// Resolve GCS object paths → media-proxy URLs the browser can fetch.
	// External URLs (WhatsApp/Telegram CDNs) are passed through unchanged.
	for i := range messages {
		if messages[i].MediaURL == nil {
			continue
		}
		if url, ok := h.mediaURLFor(c.Request.Context(), acctID, *messages[i].MediaURL); ok {
			messages[i].MediaURL = &url
		}
	}

	// Attachments ride along with the transcript in ONE batched query.
	// Fetching them per bubble would be an N+1: a conversation with fifty
	// emails would fire fifty requests, and the client would still have to
	// wait for all of them before the thread looked finished.
	if (h.attachmentRepo != nil || h.riskRepo != nil) && len(messages) > 0 {
		ids := make([]string, 0, len(messages))
		for _, m := range messages {
			ids = append(ids, m.ID)
		}

		// Both side lookups degrade independently: the transcript is the
		// content, and losing it because an enrichment query broke is the
		// worse outcome by far.
		var byMessage map[string][]*domain.MessageAttachment
		if h.attachmentRepo != nil {
			var err error
			if byMessage, err = h.attachmentRepo.ListByMessages(c.Request.Context(), ids); err != nil {
				slog.Error("failed to load message attachments", "conversation_id", convID, "error", err)
			}
		}
		var riskByMessage map[string]*domain.EmailRiskRecord
		if h.riskRepo != nil {
			var err error
			if riskByMessage, err = h.riskRepo.ListByMessages(c.Request.Context(), ids); err != nil {
				slog.Error("failed to load message risk", "conversation_id", convID, "error", err)
			}
		}

		out := make([]messageWithAttachments, len(messages))
		for i, m := range messages {
			out[i] = messageWithAttachments{
				Message:     m,
				Attachments: attachmentsOrEmpty(byMessage[m.ID]),
			}
			if rec := riskByMessage[m.ID]; rec != nil {
				out[i].Risk = &rec.Assessment
			}
		}
		c.JSON(http.StatusOK, out)
		return
	}

	c.JSON(http.StatusOK, messages)
}

// messageWithAttachments is the transcript row when attachments are
// available. The message's own fields stay at the top level (embedded
// pointer) so a client that knows nothing about attachments keeps working
// unchanged.
type messageWithAttachments struct {
	*domain.Message
	Attachments []*domain.MessageAttachment `json:"attachments"`
	// Risk is present only when something was found. Absent means nothing
	// was — the clean case is not stored, so there is nothing to send.
	Risk *domain.EmailRiskAssessment `json:"risk,omitempty"`
}

// attachmentsOrEmpty never returns nil: a JSON null and an empty list mean
// the same thing to a reader and different things to a parser.
func attachmentsOrEmpty(in []*domain.MessageAttachment) []*domain.MessageAttachment {
	if in == nil {
		return []*domain.MessageAttachment{}
	}
	return in
}

// HandleListConversationMedia godoc
// @Summary      List media (images/videos/audios/documents) of a conversation
// @Description  Returns every message in the conversation that carries
// @Description  a media_url, newest first. Drives the "Arquivos" tab on
// @Description  the contact detail sheet. GCS object_paths are resolved
// @Description  to media-proxy URLs on each request.
// @Tags         conversations
// @Produce      json
// @Param        id     path  string true  "Conversation ID"
// @Param        limit  query int    false "Maximum number of results (1-200)" default(100)
// @Param        offset query int    false "Number of results to skip" default(0)
// @Success      200  {array}   domain.Message
// @Failure      404  {object}  domain.AppError
// @Security     BearerAuth
// @Router       /conversations/{id}/media [get]
func (h *ConversationHandler) HandleListConversationMedia(c *gin.Context) {
	convID := c.Param("id")
	acctID := getAccountID(c)
	if acctID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "AUTH_UNAUTHORIZED", Message: "unauthorized"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 200 {
		limit = 200
	}
	if limit < 1 {
		limit = 1
	}
	if offset < 0 {
		offset = 0
	}

	conv, err := h.convRepo.GetByID(c.Request.Context(), convID, acctID)
	if err != nil || conv == nil {
		c.JSON(http.StatusNotFound, domain.ErrConversationNotFound)
		return
	}

	messages, err := h.msgRepo.ListConversationMedia(c.Request.Context(), convID, int32(limit), int32(offset))
	if err != nil {
		slog.Error("failed to list conversation media",
			"conversation_id", convID, "error", err)
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to list media"})
		return
	}

	for i := range messages {
		if messages[i].MediaURL == nil {
			continue
		}
		if url, ok := h.mediaURLFor(c.Request.Context(), acctID, *messages[i].MediaURL); ok {
			messages[i].MediaURL = &url
		}
	}

	// Email files live in message_attachments, not media_url — without this
	// enrichment the Arquivos tab showed chat media but no email attachment.
	// Same batched shape as the transcript.
	if h.attachmentRepo != nil && len(messages) > 0 {
		ids := make([]string, 0, len(messages))
		for _, m := range messages {
			ids = append(ids, m.ID)
		}
		byMessage, attErr := h.attachmentRepo.ListByMessages(c.Request.Context(), ids)
		if attErr != nil {
			slog.Error("failed to load media attachments", "conversation_id", convID, "error", attErr)
		} else {
			out := make([]messageWithAttachments, len(messages))
			for i, m := range messages {
				out[i] = messageWithAttachments{
					Message:     m,
					Attachments: attachmentsOrEmpty(byMessage[m.ID]),
				}
			}
			c.JSON(http.StatusOK, out)
			return
		}
	}

	c.JSON(http.StatusOK, messages)
}

// HandleGetContactStats godoc
// @Summary      Lifetime stats for a conversation's contact
// @Description  Aggregates total conversations, total messages, first
// @Description  and last interaction timestamps for the phone behind a
// @Description  conversation. Used by the contact detail sheet.
// @Tags         conversations
// @Produce      json
// @Param        id  path  string  true  "Conversation ID"
// @Success      200  {object}  domain.ContactStats
// @Failure      404  {object}  domain.AppError
// @Security     BearerAuth
// @Router       /conversations/{id}/contact-stats [get]
func (h *ConversationHandler) HandleGetContactStats(c *gin.Context) {
	convID := c.Param("id")
	acctID := getAccountID(c)
	if acctID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "AUTH_UNAUTHORIZED", Message: "unauthorized"})
		return
	}

	conv, err := h.convRepo.GetByID(c.Request.Context(), convID, acctID)
	if err != nil || conv == nil {
		c.JSON(http.StatusNotFound, domain.ErrConversationNotFound)
		return
	}

	stats, err := h.convRepo.GetContactStats(c.Request.Context(), acctID, conv.ContactID)
	if err != nil {
		slog.Error("failed to load contact stats",
			"conversation_id", convID, "contact_id", conv.ContactID, "error", err)
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to load contact stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// HandleAssignConversation assigns a conversation to a team member.

// HandleConversationBrief serves GET /v1/conversations/:id/summary.
//
// Returns 204 when the thread is too short to be worth summarising: below a
// handful of messages the summary would be longer than what it replaces, and
// an empty body says that more honestly than an empty string would.
//
// @Summary      Human-readable summary of a conversation
// @Tags         conversations
// @Produce      json
// @Param        id path string true "Conversation ID"
// @Success      200 {object} domain.ConversationBrief
// @Success      204 "thread too short to summarise"
// @Failure      404 {object} domain.AppError
// @Security     BearerAuth
// @Router       /conversations/{id}/summary [get]
func (h *ConversationHandler) HandleConversationBrief(c *gin.Context) {
	if h.briefUC == nil {
		respondError(c, &domain.AppError{
			Code: "CONVERSATION_BRIEF_UNAVAILABLE", Message: "summaries are not configured"})
		return
	}
	brief, err := h.briefUC.Get(c.Request.Context(), actorFrom(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	if brief == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, brief)
}
