package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// SetProspectingUC wires the optional prospecting use case (US-022). Kept off
// the constructor so the NewFunnelHandler signature — and the router.Setup call
// site — stay untouched (per the optional-setter wiring pattern).
func (h *FunnelHandler) SetProspectingUC(uc *usecases.ProspectingUseCase) {
	h.prospectingUC = uc
}

type prospectingReq struct {
	Query        string  `json:"query"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	RadiusMeters float64 `json:"radius_meters"`
	LanguageCode string  `json:"language_code"`
}

// HandleProspecting serves POST /v1/funnels/:id/prospecting — runs a Google
// Places search for the funnel and turns the businesses into prospected leads.
func (h *FunnelHandler) HandleProspecting(c *gin.Context) {
	if h.prospectingUC == nil {
		respondError(c, domain.ErrProspectingNotConfigured)
		return
	}
	accountID := getAccountID(c)
	var req prospectingReq
	if !bindJSON(c, &req) {
		return
	}

	res, err := h.prospectingUC.Execute(c.Request.Context(), usecases.ProspectingSearchDTO{
		AccountID:    accountID,
		FunnelID:     c.Param("id"),
		Query:        req.Query,
		Latitude:     req.Latitude,
		Longitude:    req.Longitude,
		RadiusMeters: req.RadiusMeters,
		LanguageCode: req.LanguageCode,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// HandlePreviewProspecting serves POST /v1/funnels/:id/prospecting/preview.
//
// Runs the search and creates NOTHING. Before this endpoint a search WAS an
// import: a bad keyword spent the month's lead allowance on a list nobody
// would have picked, and the allowance is the part the customer pays for.
//
// @Summary  Preview what a prospecting search would import
// @Tags     funnels
// @Accept   json
// @Produce  json
// @Param    id path string true "Funnel ID"
// @Success  200 {object} usecases.ProspectingPreview
// @Failure  400 {object} domain.AppError
// @Router   /funnels/{id}/prospecting/preview [post]
func (h *FunnelHandler) HandlePreviewProspecting(c *gin.Context) {
	if h.prospectingUC == nil {
		respondError(c, domain.ErrProspectingNotConfigured)
		return
	}
	var req prospectingReq
	if !bindJSON(c, &req) {
		return
	}

	preview, err := h.prospectingUC.Preview(c.Request.Context(), usecases.ProspectingSearchDTO{
		AccountID:    getAccountID(c),
		FunnelID:     c.Param("id"),
		Query:        req.Query,
		Latitude:     req.Latitude,
		Longitude:    req.Longitude,
		RadiusMeters: req.RadiusMeters,
		LanguageCode: req.LanguageCode,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, preview)
}

type prospectingImportReq struct {
	// Selected carries the candidates back rather than a search id:
	// re-running the search would cost another call to a paid directory and
	// could return a different list, so the person would import something
	// other than what they ticked.
	Selected []usecases.PreviewCandidate `json:"selected" binding:"required"`
}

// HandleImportProspecting serves POST /v1/funnels/:id/prospecting/import.
//
// @Summary  Import chosen prospecting results as leads
// @Tags     funnels
// @Accept   json
// @Produce  json
// @Param    id path string true "Funnel ID"
// @Success  200 {object} usecases.ProspectingResult
// @Failure  400 {object} domain.AppError
// @Router   /funnels/{id}/prospecting/import [post]
func (h *FunnelHandler) HandleImportProspecting(c *gin.Context) {
	if h.prospectingUC == nil {
		respondError(c, domain.ErrProspectingNotConfigured)
		return
	}
	var req prospectingImportReq
	if !bindJSON(c, &req) {
		return
	}
	if len(req.Selected) == 0 {
		respondError(c, &domain.AppError{
			Code: "VALIDATION_ERROR", Message: "selecione ao menos uma empresa"})
		return
	}

	res, err := h.prospectingUC.ImportSelected(c.Request.Context(),
		usecases.ProspectingSearchDTO{
			AccountID: getAccountID(c),
			FunnelID:  c.Param("id"),
		}, req.Selected)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}
