package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// The first offer (onboarding, last step — 2026-08-18).

// Hangs off MeHandler by optional setter, same as the command centre and the
// dashboard BFF, so neither NewMeHandler nor router.Setup changes signature.
// Unwired answers 503 rather than panicking.

// SetFirstOfferUC wires the onboarding's last step.
func (h *MeHandler) SetFirstOfferUC(uc *usecases.FirstOfferUseCase) {
	h.firstOfferUC = uc
}

// HandleFirstOffer serves POST /v1/me/onboarding/first-offer.
//
// @Summary     Write the first offer
// @Description Finds (or makes) the account's first lead — one of the customers she imported, else one business in her territory — drafts an approach for it through the same path the daily product uses, and returns lead + draft. Idempotent: a reload gets the same offer. Nothing is sent.
// @Tags        Me
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} usecases.FirstOfferResult
// @Failure     401 {object} domain.AppError
// @Failure     503 {object} domain.AppError "FIRST_OFFER_UNAVAILABLE"
// @Router      /me/onboarding/first-offer [post]
func (h *MeHandler) HandleFirstOffer(c *gin.Context) {
	if h.firstOfferUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "FIRST_OFFER_UNAVAILABLE", Message: "first offer not configured",
		})
		return
	}
	accountID := c.GetString("accountID")
	userID := c.GetString("userID")
	if accountID == "" || userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}
	res, err := h.firstOfferUC.Execute(c.Request.Context(), accountID, userID)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}
