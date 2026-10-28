package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

func (h *ConversationHandler) HandleSearchMessages(c *gin.Context) {
	accountID := c.GetString("accountID")
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: "query parameter 'q' is required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	results, err := h.msgRepo.SearchMessages(c.Request.Context(), accountID, query, limit, offset)
	if err != nil {
		slog.Error("search messages failed", "error", err)
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "search failed"})
		return
	}
	if results == nil {
		results = []*domain.MessageSearchResult{}
	}
	c.JSON(http.StatusOK, results)
}

// BulkActionRequest represents the request body for bulk conversation actions.
type BulkActionRequest struct {
	Action          string            `json:"action" binding:"required"`
	ConversationIDs []string          `json:"conversation_ids" binding:"required"`
	Params          map[string]string `json:"params,omitempty"`
}

// BulkActionResult represents the outcome for a single conversation in a bulk operation.
type BulkActionResult struct {
	ConversationID string `json:"conversation_id"`
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
}

// HandleBulkAction performs bulk operations on multiple conversations.

// HandleBulkAction performs bulk operations on multiple conversations.
func (h *ConversationHandler) HandleBulkAction(c *gin.Context) {
	accountID := c.GetString("accountID")

	var req BulkActionRequest
	if !bindJSON(c, &req) {
		return
	}

	// Validate action
	switch req.Action {
	case "resolve", "assign", "set_priority":
	default:
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_ACTION", Message: "action must be resolve, assign, or set_priority"})
		return
	}

	// Validate params for actions that require them
	if req.Action == "assign" {
		if _, ok := req.Params["user_id"]; !ok {
			c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: "params.user_id is required for assign action"})
			return
		}
	}
	if req.Action == "set_priority" {
		p, ok := req.Params["priority"]
		if !ok {
			c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: "params.priority is required for set_priority action"})
			return
		}
		switch p {
		case "low", "normal", "high", "urgent":
		default:
			c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_PRIORITY", Message: "priority must be low, normal, high, or urgent"})
			return
		}
	}

	if len(req.ConversationIDs) == 0 {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: "conversation_ids must not be empty"})
		return
	}
	if len(req.ConversationIDs) > 100 {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: "conversation_ids must not exceed 100 items"})
		return
	}

	ctx := c.Request.Context()
	results := make([]BulkActionResult, 0, len(req.ConversationIDs))

	for _, convID := range req.ConversationIDs {
		result := BulkActionResult{ConversationID: convID}

		// Verify ownership by checking account_id matches
		conv, err := h.convRepo.GetByID(ctx, convID, accountID)
		if err != nil || conv == nil {
			result.Error = "conversation not found"
			results = append(results, result)
			continue
		}

		switch req.Action {
		case "resolve":
			now := time.Now()
			conv.Status = domain.ConversationClosed
			conv.ResolvedAt = &now
			if err := h.convRepo.Update(ctx, conv); err != nil {
				result.Error = "failed to resolve conversation"
				results = append(results, result)
				continue
			}

		case "assign":
			userID := req.Params["user_id"]
			conv.AssignedTo = &userID
			if err := h.convRepo.Update(ctx, conv); err != nil {
				result.Error = "failed to assign conversation"
				results = append(results, result)
				continue
			}

		case "set_priority":
			conv.Priority = req.Params["priority"]
			if err := h.convRepo.Update(ctx, conv); err != nil {
				result.Error = "failed to set priority"
				results = append(results, result)
				continue
			}
		}

		result.Success = true
		results = append(results, result)
	}

	// Broadcast updates via WebSocket + tenant outbound webhooks. Both
	// fan-outs only fire for successful rows so a partial failure
	// doesn't leak fake state changes.
	for _, r := range results {
		if !r.Success {
			continue
		}
		if h.hub != nil {
			event, _ := json.Marshal(map[string]interface{}{
				"event":           "conversation_updated",
				"conversation_id": r.ConversationID,
				"bulk_action":     req.Action,
			})
			h.hub.BroadcastToAccount(accountID, event)
		}
		if h.events != nil {
			switch req.Action {
			case "resolve":
				h.events.Emit(accountID, "conversation.closed", map[string]any{
					"conversation_id": r.ConversationID,
					"bulk":            true,
				})
			case "assign":
				h.events.Emit(accountID, "conversation.assigned", map[string]any{
					"conversation_id": r.ConversationID,
					"assigned_to":     req.Params["user_id"],
					"bulk":            true,
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}
