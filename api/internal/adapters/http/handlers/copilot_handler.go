package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// In-app copilot (US-M026).

type CopilotHandler struct {
	uc *usecases.CopilotUseCase
}

func NewCopilotHandler(uc *usecases.CopilotUseCase) *CopilotHandler {
	return &CopilotHandler{uc: uc}
}

type copilotAskReq struct {
	Message string `json:"message" binding:"required"`
	// History is the prior turns, oldest first. The client owns the
	// transcript: the copilot is a tool, not a record, and persisting
	// every exploratory question server-side would be a data-retention
	// liability nobody asked for.
	History []copilotTurn `json:"history"`
}

type copilotTurn struct {
	Role    string `json:"role"` // "user" | "model"
	Content string `json:"content"`
}

// maxCopilotHistory bounds what a client can replay at us — an unbounded
// history is an unbounded prompt, billed to the account.
const maxCopilotHistory = 20

// HandleAsk serves POST /v1/copilot/ask.
func (h *CopilotHandler) HandleAsk(c *gin.Context) {
	if h.uc == nil {
		respondError(c, &domain.AppError{
			Code:    "COPILOT_UNAVAILABLE",
			Message: "copiloto não configurado",
		})
		return
	}
	var req copilotAskReq
	if !bindJSON(c, &req) {
		return
	}
	history := req.History
	if len(history) > maxCopilotHistory {
		history = history[len(history)-maxCopilotHistory:]
	}
	turns := make([]domain.ChatMessage, 0, len(history))
	for _, t := range history {
		role := "user"
		if t.Role == "model" || t.Role == "assistant" {
			role = "model"
		}
		turns = append(turns, domain.ChatMessage{Role: role, Content: t.Content})
	}

	reply, err := h.uc.Ask(c.Request.Context(), actorFrom(c), turns, req.Message)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, reply)
}
