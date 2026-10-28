package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

type NewsletterHandler struct {
	subscribeUC *usecases.SubscribeNewsletterUseCase
}

func NewNewsletterHandler(subscribeUC *usecases.SubscribeNewsletterUseCase) *NewsletterHandler {
	return &NewsletterHandler{subscribeUC: subscribeUC}
}

type subscribeRequest struct {
	Email string `json:"email" binding:"required"`
	// Website is a honeypot field. The marketing form keeps it hidden, so a
	// non-empty value means a bot filled it in. See domain.ErrSpamDetected.
	Website string `json:"website"`
}

func (h *NewsletterHandler) HandleSubscribe(c *gin.Context) {
	var req subscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: "email is required"})
		return
	}

	err := h.subscribeUC.Execute(c.Request.Context(), req.Email, req.Website)
	switch err {
	case nil, domain.ErrSpamDetected:
		// On a tripped honeypot we still return 200 (without persisting) so the
		// bot cannot distinguish the trap from a real subscription.
		c.JSON(http.StatusOK, gin.H{"message": "subscribed"})
	case domain.ErrInvalidEmail:
		c.JSON(http.StatusBadRequest, err)
	case domain.ErrTooManyRequests:
		c.JSON(http.StatusTooManyRequests, err)
	default:
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to subscribe"})
	}
}
