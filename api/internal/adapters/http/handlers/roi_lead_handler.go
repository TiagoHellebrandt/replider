package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

type ROIHandler struct {
	submitUC *usecases.SubmitROILeadUseCase
}

func NewROIHandler(submitUC *usecases.SubmitROILeadUseCase) *ROIHandler {
	return &ROIHandler{submitUC: submitUC}
}

type roiLeadRequest struct {
	Email            string  `json:"email" binding:"required"`
	MessagesPerMonth int     `json:"messages_per_month" binding:"required"`
	CurrentCost      float64 `json:"current_cost" binding:"required"`
	HourlyRate       float64 `json:"hourly_rate" binding:"required"`
	// Website is a honeypot field. The marketing form keeps it hidden, so a
	// non-empty value means a bot filled it in. See domain.ErrSpamDetected.
	Website string `json:"website"`
}

// HandleSubmit captures a lead from the public ROI calculator and returns the
// estimated monthly savings so the client can reveal it past the email gate
// (US-163).
func (h *ROIHandler) HandleSubmit(c *gin.Context) {
	var req roiLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: "email, messages_per_month, current_cost and hourly_rate are required"})
		return
	}

	savings, err := h.submitUC.Execute(c.Request.Context(), usecases.SubmitROILeadInput{
		Email:            req.Email,
		MessagesPerMonth: req.MessagesPerMonth,
		CurrentCost:      req.CurrentCost,
		HourlyRate:       req.HourlyRate,
		Honeypot:         req.Website,
	})
	switch err {
	case nil:
		c.JSON(http.StatusOK, gin.H{"estimated_savings": savings})
	case domain.ErrSpamDetected:
		// On a tripped honeypot we still return 200 (without persisting, and
		// with a zeroed estimate) so the bot cannot distinguish the trap from a
		// real submission.
		c.JSON(http.StatusOK, gin.H{"estimated_savings": 0})
	case domain.ErrInvalidEmail, domain.ErrMissingROIFields:
		c.JSON(http.StatusBadRequest, err)
	case domain.ErrTooManyRequests:
		c.JSON(http.StatusTooManyRequests, err)
	default:
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to submit"})
	}
}
