package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

type CannedResponseHandler struct {
	uc *usecases.CannedResponseUseCase
}

func NewCannedResponseHandler(uc *usecases.CannedResponseUseCase) *CannedResponseHandler {
	return &CannedResponseHandler{uc: uc}
}

func respondCannedError(c *gin.Context, err error, fallback string) {
	switch err {
	case domain.ErrCannedResponseNotFound:
		c.JSON(http.StatusNotFound, err)
	case domain.ErrCannedResponseShortcutExists:
		c.JSON(http.StatusConflict, err)
	case domain.ErrCannedResponseForbidden:
		c.JSON(http.StatusForbidden, err)
	default:
		if appErr, ok := err.(*domain.AppError); ok {
			c.JSON(http.StatusBadRequest, appErr)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: fallback})
	}
}

type CreateCannedResponseReq struct {
	Shortcut string `json:"shortcut" binding:"required"`
	Title    string `json:"title" binding:"required"`
	Content  string `json:"content" binding:"required"`
	Category string `json:"category"`
	// Visibility: 'account' (default) or 'personal' (US-018).
	Visibility string `json:"visibility" binding:"omitempty,oneof=account personal"`
}

func (h *CannedResponseHandler) HandleCreate(c *gin.Context) {
	accountID := c.GetString("accountID")
	var req CreateCannedResponseReq
	if !bindJSON(c, &req) {
		return
	}

	cr, err := h.uc.Create(c.Request.Context(), usecases.CreateCannedResponseDTO{
		AccountID:  accountID,
		UserID:     c.GetString("userID"),
		Role:       c.GetString("role"),
		Shortcut:   req.Shortcut,
		Title:      req.Title,
		Content:    req.Content,
		Category:   req.Category,
		Visibility: req.Visibility,
	})
	if err != nil {
		respondCannedError(c, err, "failed to create canned response")
		return
	}

	c.JSON(http.StatusCreated, cr)
}

func (h *CannedResponseHandler) HandleList(c *gin.Context) {
	accountID := c.GetString("accountID")
	responses, err := h.uc.List(c.Request.Context(), accountID, c.GetString("userID"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to list canned responses"})
		return
	}
	c.JSON(http.StatusOK, responses)
}

type UpdateCannedResponseReq struct {
	Shortcut string `json:"shortcut" binding:"required"`
	Title    string `json:"title" binding:"required"`
	Content  string `json:"content" binding:"required"`
	Category string `json:"category"`
}

func (h *CannedResponseHandler) HandleUpdate(c *gin.Context) {
	accountID := c.GetString("accountID")
	id := c.Param("id")

	var req UpdateCannedResponseReq
	if !bindJSON(c, &req) {
		return
	}

	cr, err := h.uc.Update(c.Request.Context(), usecases.UpdateCannedResponseDTO{
		AccountID: accountID,
		UserID:    c.GetString("userID"),
		Role:      c.GetString("role"),
		ID:        id,
		Shortcut:  req.Shortcut,
		Title:     req.Title,
		Content:   req.Content,
		Category:  req.Category,
	})
	if err != nil {
		respondCannedError(c, err, "failed to update canned response")
		return
	}

	c.JSON(http.StatusOK, cr)
}

func (h *CannedResponseHandler) HandleDelete(c *gin.Context) {
	accountID := c.GetString("accountID")
	id := c.Param("id")

	if err := h.uc.Delete(c.Request.Context(), accountID, c.GetString("userID"), c.GetString("role"), id); err != nil {
		respondCannedError(c, err, "failed to delete canned response")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "canned response deleted"})
}
