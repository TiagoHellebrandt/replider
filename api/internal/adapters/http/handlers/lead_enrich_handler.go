package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// HandleEnrich serves POST /v1/leads/:id/enrich — kicks off asynchronous
// decision-maker enrichment via Gemini Grounding (US-023). It returns 202
// Accepted immediately; the lead moves enriching → enriched in the background.
// Synchronous errors (ownership, ENRICHMENT_REQUIRES_GEMINI, BYOK_NOT_CONFIGURED)
// are returned before the 202.
// @Summary     Research a lead's company
// @Description Answers 202 and runs a grounded search in the background, writing the decision maker and their public profiles onto the lead. Requires a Gemini provider.
// @Tags        Leads
// @Produce     json
// @Param       id path string true "Lead ID"
// @Success     202
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     404 {object} domain.AppError
// @Failure     503 {object} domain.AppError
// @Security    BearerAuth
// @Router      /leads/{id}/enrich [post]
func (h *LeadHandler) HandleEnrich(c *gin.Context) {
	if h.enrichUC == nil {
		respondError(c, &domain.AppError{Code: "ENRICHMENT_NOT_CONFIGURED", Message: "lead enrichment is not configured"})
		return
	}
	accountID := getAccountID(c)
	if !h.ensureLeadMutable(c) {
		return
	}
	if err := h.enrichUC.Start(c.Request.Context(), accountID, c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "enriching", "lead_id": c.Param("id")})
}
