package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
)

// SmartReplier is the contract the conversation handler uses for
// reply-suggestion fan-out. We keep the interface in the handlers
// package (instead of importing services.SmartReplier directly into
// the struct field) so test wiring can inject a fake without pulling
// in the LLM provider chain.
type SmartReplier interface {
	Suggest(ctx context.Context, accountID string, history []services.SmartReplyHistoryItem, language, agentSystemPrompt string) ([]string, error)
}

// ConversationSummarizer condenses a thread to one sentence so the
// attendant can pick up a long conversation without scrolling. Same
// adapter→service interface trick as SmartReplier above.
type ConversationSummarizer interface {
	Summarize(ctx context.Context, accountID string, history []services.SummaryHistoryItem, language, agentSystemPrompt string) (string, error)
}

// SetSmartReplier wires the LLM-backed reply suggester. Without it
// the suggest endpoint returns 503 — the rest of the conversation
// surface keeps working.
func (h *ConversationHandler) SetSmartReplier(s SmartReplier) {
	h.smartReplier = s
}

// SetSummarizer wires the LLM-backed thread summarizer.
func (h *ConversationHandler) SetSummarizer(s ConversationSummarizer) {
	h.summarizer = s
}

// HandleSummarize returns a one-sentence digest of the conversation.
// Used by the dashboard "ler resumo" affordance for long threads.
//
// @Summary  Summarize a conversation
// @Tags     conversations
// @Router   /conversations/{id}/summary [post]
func (h *ConversationHandler) HandleSummarize(c *gin.Context) {
	if h.summarizer == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code:    "SUMMARY_DISABLED",
			Message: "summarizer not configured",
		})
		return
	}
	accountID := c.GetString("accountID")
	convID := c.Param("id")
	conv, err := h.convRepo.GetByID(c.Request.Context(), convID, accountID)
	if err != nil {
		respondError(c, err)
		return
	}
	if conv == nil {
		c.JSON(http.StatusNotFound, domain.ErrConversationNotFound)
		return
	}
	// Wider window than smart-reply: summary needs the full arc, not
	// just the recent reply context. 50 messages comfortably covers
	// most support threads while keeping the prompt under 8k tokens
	// on Gemini Flash.
	msgs, err := h.msgRepo.ListByConversationID(c.Request.Context(), convID, 50, 0)
	if err != nil {
		respondError(c, err)
		return
	}
	history := make([]services.SummaryHistoryItem, 0, len(msgs))
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		from := "agent"
		if m.Direction == "inbound" {
			from = "customer"
		}
		content := ""
		if m.Content != nil {
			content = *m.Content
		}
		if content == "" {
			continue
		}
		history = append(history, services.SummaryHistoryItem{
			From: from, Content: content,
		})
	}
	if len(history) == 0 {
		c.JSON(http.StatusOK, gin.H{"summary": ""})
		return
	}
	systemPrompt := ""
	if conv.AgentID != nil && h.agentRepo != nil {
		if agent, err := h.agentRepo.GetByID(c.Request.Context(), *conv.AgentID); err == nil && agent != nil {
			if agent.SystemPrompt != nil {
				systemPrompt = *agent.SystemPrompt
			}
		}
	}
	language := ""
	if conv.Language != nil {
		language = *conv.Language
	}
	out, err := h.summarizer.Summarize(c.Request.Context(), getAccountID(c), history, language, systemPrompt)
	if errors.Is(err, domain.ErrBYOKKeyMissing) {
		respondErr(c, err)
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, domain.AppError{
			Code: "SUMMARY_FAILED", Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"summary": out})
}

// HandleSuggestReplies returns up to 3 short reply candidates the
// human attendant can pick from. Saves typing on the most-common
// "thanks for reaching out" / "could you share X" prefixes without
// committing to an automated reply.
//
// @Summary  Suggest reply drafts for a conversation
// @Tags     conversations
// @Produce  json
// @Security BearerAuth
// @Param    id  path  string  true  "Conversation ID"
// @Success  200  {object}  map[string][]string  "{suggestions: [...]}"
// @Router   /conversations/{id}/suggest-replies [post]
func (h *ConversationHandler) HandleSuggestReplies(c *gin.Context) {
	if h.smartReplier == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code:    "SMART_REPLY_DISABLED",
			Message: "smart reply provider not configured",
		})
		return
	}
	accountID := c.GetString("accountID")
	convID := c.Param("id")

	conv, err := h.convRepo.GetByID(c.Request.Context(), convID, accountID)
	if err != nil {
		respondError(c, err)
		return
	}
	if conv == nil {
		c.JSON(http.StatusNotFound, domain.ErrConversationNotFound)
		return
	}
	// Pull the last 12 messages — wide enough to capture a typical
	// support back-and-forth, narrow enough to keep token cost low.
	msgs, err := h.msgRepo.ListByConversationID(c.Request.Context(), convID, 12, 0)
	if err != nil {
		respondError(c, err)
		return
	}
	if len(msgs) == 0 {
		c.JSON(http.StatusOK, gin.H{"suggestions": []string{}})
		return
	}
	// ListByConversationID returns newest-first; reverse so the LLM
	// sees the actual chronological order.
	history := make([]services.SmartReplyHistoryItem, 0, len(msgs))
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		from := "agent"
		if m.Direction == "inbound" {
			from = "customer"
		}
		content := ""
		if m.Content != nil {
			content = *m.Content
		}
		// Skip empty / pure-media rows — the LLM has no signal there
		// and an empty role can confuse the chat-completion call.
		if content == "" {
			continue
		}
		history = append(history, services.SmartReplyHistoryItem{
			From:    from,
			Content: content,
		})
	}
	if len(history) == 0 {
		c.JSON(http.StatusOK, gin.H{"suggestions": []string{}})
		return
	}

	// Pull the agent's system prompt when the conversation is bound
	// to one — keeps suggestions in-brand. Best-effort; nil agent
	// (human-only channel) falls back to the SmartReplier default.
	systemPrompt := ""
	if conv.AgentID != nil && h.agentRepo != nil {
		if agent, err := h.agentRepo.GetByID(c.Request.Context(), *conv.AgentID); err == nil && agent != nil {
			if agent.SystemPrompt != nil {
				systemPrompt = *agent.SystemPrompt
			}
		}
	}

	language := ""
	if conv.Language != nil {
		language = *conv.Language
	}
	out, err := h.smartReplier.Suggest(c.Request.Context(), getAccountID(c), history, language, systemPrompt)
	if errors.Is(err, domain.ErrBYOKKeyMissing) {
		respondErr(c, err)
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, domain.AppError{
			Code:    "SMART_REPLY_FAILED",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"suggestions": out})
}
