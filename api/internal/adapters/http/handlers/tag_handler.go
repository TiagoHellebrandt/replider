package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

type TagHandler struct {
	uc *usecases.TagUseCase
}

func NewTagHandler(uc *usecases.TagUseCase) *TagHandler {
	return &TagHandler{uc: uc}
}

type CreateTagReq struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color"`
}

func (h *TagHandler) HandleCreate(c *gin.Context) {
	accountID := c.GetString("accountID")
	var req CreateTagReq
	if !bindJSON(c, &req) {
		return
	}

	tag, err := h.uc.Create(c.Request.Context(), usecases.CreateTagDTO{
		AccountID: accountID,
		Name:      req.Name,
		Color:     req.Color,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to create tag"})
		return
	}

	c.JSON(http.StatusCreated, tag)
}

func (h *TagHandler) HandleList(c *gin.Context) {
	accountID := c.GetString("accountID")
	tags, err := h.uc.List(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to list tags"})
		return
	}
	c.JSON(http.StatusOK, tags)
}

type UpdateTagReq struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color"`
}

func (h *TagHandler) HandleUpdate(c *gin.Context) {
	accountID := c.GetString("accountID")
	id := c.Param("id")

	var req UpdateTagReq
	if !bindJSON(c, &req) {
		return
	}

	tag, err := h.uc.Update(c.Request.Context(), usecases.UpdateTagDTO{
		AccountID: accountID,
		ID:        id,
		Name:      req.Name,
		Color:     req.Color,
	})
	if err != nil {
		if err == domain.ErrTagNotFound {
			c.JSON(http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to update tag"})
		return
	}

	c.JSON(http.StatusOK, tag)
}

func (h *TagHandler) HandleDelete(c *gin.Context) {
	accountID := c.GetString("accountID")
	id := c.Param("id")

	if err := h.uc.Delete(c.Request.Context(), accountID, id); err != nil {
		if err == domain.ErrTagNotFound {
			c.JSON(http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to delete tag"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tag deleted"})
}

type AddTagReq struct {
	TagID string `json:"tag_id" binding:"required"`
}

func (h *TagHandler) HandleAddToConversation(c *gin.Context) {
	accountID := c.GetString("accountID")
	conversationID := c.Param("id")

	var req AddTagReq
	if !bindJSON(c, &req) {
		return
	}

	if err := h.uc.AddToConversation(c.Request.Context(), accountID, conversationID, req.TagID); err != nil {
		if err == domain.ErrConversationNotFound || err == domain.ErrTagNotFound {
			c.JSON(http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to add tag"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tag added"})
}

func (h *TagHandler) HandleRemoveFromConversation(c *gin.Context) {
	accountID := c.GetString("accountID")
	conversationID := c.Param("id")
	tagID := c.Param("tagId")

	if err := h.uc.RemoveFromConversation(c.Request.Context(), accountID, conversationID, tagID); err != nil {
		if err == domain.ErrConversationNotFound {
			c.JSON(http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to remove tag"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tag removed"})
}

func (h *TagHandler) HandleListConversationTags(c *gin.Context) {
	accountID := c.GetString("accountID")
	conversationID := c.Param("id")

	tags, err := h.uc.ListByConversation(c.Request.Context(), accountID, conversationID)
	if err != nil {
		if err == domain.ErrConversationNotFound {
			c.JSON(http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to list tags"})
		return
	}

	c.JSON(http.StatusOK, tags)
}
