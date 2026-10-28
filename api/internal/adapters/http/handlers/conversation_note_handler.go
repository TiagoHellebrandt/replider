package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

func (h *ConversationHandler) HandleCreateNote(c *gin.Context) {
	accountID := c.GetString("accountID")
	userID := c.GetString("userID")
	userName := c.GetString("userName")
	convID := c.Param("id")

	conv, err := h.convRepo.GetByID(c.Request.Context(), convID, accountID)
	if err != nil || conv == nil {
		c.JSON(http.StatusNotFound, domain.ErrConversationNotFound)
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: err.Error()})
		return
	}

	now := time.Now()
	note := &domain.ConversationNote{
		ID:             ulid.Make().String(),
		ConversationID: convID,
		UserID:         userID,
		UserName:       userName,
		Content:        req.Content,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := h.noteRepo.Create(c.Request.Context(), note); err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to create note"})
		return
	}
	c.JSON(http.StatusCreated, note)
}

func (h *ConversationHandler) HandleListNotes(c *gin.Context) {
	accountID := c.GetString("accountID")
	convID := c.Param("id")

	conv, err := h.convRepo.GetByID(c.Request.Context(), convID, accountID)
	if err != nil || conv == nil {
		c.JSON(http.StatusNotFound, domain.ErrConversationNotFound)
		return
	}

	notes, err := h.noteRepo.ListByConversationID(c.Request.Context(), convID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to list notes"})
		return
	}
	if notes == nil {
		notes = []*domain.ConversationNote{}
	}
	c.JSON(http.StatusOK, notes)
}

func (h *ConversationHandler) HandleDeleteNote(c *gin.Context) {
	noteID := c.Param("noteId")

	if err := h.noteRepo.Delete(c.Request.Context(), noteID); err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to delete note"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "note deleted"})
}

// --- Full-Text Message Search ---
