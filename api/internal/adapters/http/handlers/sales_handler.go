package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

type SalesHandler struct {
	submitUC *usecases.SubmitSalesLeadUseCase
}

func NewSalesHandler(submitUC *usecases.SubmitSalesLeadUseCase) *SalesHandler {
	return &SalesHandler{submitUC: submitUC}
}

type salesLeadRequest struct {
	Company       string `json:"company" binding:"required"`
	Email         string `json:"email" binding:"required"`
	CompanySize   string `json:"company_size" binding:"required"`
	Channels      string `json:"channels" binding:"required"`
	MonthlyVolume string `json:"monthly_volume" binding:"required"`
	Message       string `json:"message"`
	// Website is a honeypot field. The marketing form keeps it hidden, so a
	// non-empty value means a bot filled it in. See domain.ErrSpamDetected.
	Website string `json:"website"`
}

// HandleSubmit captures an Enterprise-tier sales-qualification lead (US-157).
func (h *SalesHandler) HandleSubmit(c *gin.Context) {
	var req salesLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: "company, email, company_size, channels and monthly_volume are required"})
		return
	}

	err := h.submitUC.Execute(c.Request.Context(), usecases.SubmitSalesLeadInput{
		Company:       req.Company,
		Email:         req.Email,
		CompanySize:   req.CompanySize,
		Channels:      req.Channels,
		MonthlyVolume: req.MonthlyVolume,
		Message:       req.Message,
		Honeypot:      req.Website,
	})
	switch err {
	case nil, domain.ErrSpamDetected:
		// On a tripped honeypot we still return 200 (without persisting) so the
		// bot cannot distinguish the trap from a real submission.
		c.JSON(http.StatusOK, gin.H{"message": "received"})
	case domain.ErrInvalidEmail, domain.ErrMissingSalesFields:
		c.JSON(http.StatusBadRequest, err)
	case domain.ErrTooManyRequests:
		c.JSON(http.StatusTooManyRequests, err)
	default:
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to submit"})
	}
}
