package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// The Radar endpoints hang off MeHandler via optional setters (the command
// centre / dashboard BFF pattern), so neither NewMeHandler nor router.Setup
// changes signature. Unwired answers 503 rather than panicking.

// SetRadarUC wires the unified opportunity feed (Ofertas phase 2).
func (h *MeHandler) SetRadarUC(uc *usecases.RadarUseCase) { h.radarUC = uc }

// SetRadarParkOfferUC wires "Preparar oferta" for a CNES park item.
func (h *MeHandler) SetRadarParkOfferUC(uc *usecases.RadarParkOfferUseCase) {
	h.radarParkOfferUC = uc
}

// HandleGetRadar serves GET /v1/me/radar.
//
// @Summary     The opportunity radar
// @Description Everything the system detected, one feed: open tenders (PNCP), establishments in the territory holding catalogue-matched equipment (CNES, counts only — never equipment age), customers predicted to run out within a week, relationship dates within two weeks. Items are derived, never stored; a failed collector is a named warning, not an empty feed.
// @Tags        Me
// @Produce     json
// @Success     200 {object} usecases.RadarResponse
// @Failure     401 {object} domain.AppError
// @Failure     503 {object} domain.AppError
// @Security    BearerAuth
// @Router      /me/radar [get]
func (h *MeHandler) HandleGetRadar(c *gin.Context) {
	if h.radarUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "RADAR_UNAVAILABLE", Message: "radar not configured"})
		return
	}
	// ExecuteFor, not Execute: the "offer already written" mark is scoped to
	// the caller exactly as the approval queue is.
	out, err := h.radarUC.ExecuteFor(c.Request.Context(), actorFrom(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type radarParkOfferRequest struct {
	CNES string `json:"cnes" binding:"required"`
}

// HandleRadarParkOffer serves POST /v1/me/radar/park-offer.
//
// @Summary     Turn a CNES park hit into a queued offer
// @Description Creates the contact and lead (default funnel, source cnes_park, the public park facts in the enrichment) and queues an AI draft through the normal pipeline — product gate and spend caps included. Nothing is sent; the draft waits in the approval queue.
// @Tags        Me
// @Accept      json
// @Produce     json
// @Param       payload body radarParkOfferRequest true "The establishment's CNES id"
// @Success     200 {object} domain.Lead
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     503 {object} domain.AppError
// @Security    BearerAuth
// @Router      /me/radar/park-offer [post]
func (h *MeHandler) HandleRadarParkOffer(c *gin.Context) {
	if h.radarParkOfferUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "RADAR_UNAVAILABLE", Message: "radar not configured"})
		return
	}
	var req radarParkOfferRequest
	if !bindJSON(c, &req) {
		return
	}
	lead, err := h.radarParkOfferUC.Offer(c.Request.Context(), getAccountID(c), req.CNES)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, lead)
}

// SetTerritoryHuntUC wires the interactive CNES hunt behind the Radar.
func (h *MeHandler) SetTerritoryHuntUC(uc *usecases.TerritoryHuntUseCase) {
	h.territoryHuntUC = uc
}

type territoryHuntRequest struct {
	Equipment string `json:"equipment" binding:"required"`
	// UF overrides the account's territory; normally absent.
	UF string `json:"uf"`
}

// HandleTerritoryHunt serves POST /v1/me/territory/hunt.
//
// @Summary     Hunt the territory by installed equipment
// @Description Resolves the equipment name against the CNES dictionary and answers which establishments in the account's territory declare it — counts only (CNES never publishes equipment age). Hits already in the account's contacts are flagged already_known. Read-only: importing one goes through POST /me/radar/park-offer.
// @Tags        Me
// @Accept      json
// @Produce     json
// @Param       payload body territoryHuntRequest true "Equipment in the seller's words; optional UF override"
// @Success     200 {object} usecases.TerritoryHuntResult
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Failure     503 {object} domain.AppError
// @Security    BearerAuth
// @Router      /me/territory/hunt [post]
func (h *MeHandler) HandleTerritoryHunt(c *gin.Context) {
	if h.territoryHuntUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "RADAR_UNAVAILABLE", Message: "radar not configured"})
		return
	}
	var req territoryHuntRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.territoryHuntUC.Execute(c.Request.Context(), getAccountID(c), req.Equipment, req.UF)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// SetAutoHuntUC wires the catalogue-driven hunt — the one that needs no words.
func (h *MeHandler) SetAutoHuntUC(uc *usecases.AutoHuntUseCase) { h.autoHuntUC = uc }

// autoHuntResponse is what one run produced.
type autoHuntResponse struct {
	// Found is how many untouched establishments the territory offered.
	Found int `json:"found"`
	// Created is how many became leads. Lower than Found once the cap bites,
	// and the difference is not a failure — it is tomorrow's batch.
	Created int `json:"created"`
}

// HandleAutoHunt serves POST /v1/me/territory/auto-hunt.
//
// The same sweep the nightly worker runs, on demand. It exists because the
// alternative was waiting until tomorrow, or typing equipment names by hand —
// and typing names is the opposite of a product that goes looking. It reads
// the catalogue, resolves it against the CNES dictionary and fills the funnel
// with the biggest untouched parks in the territory.
//
// It creates leads and writes nothing else: no approach is drafted, nothing is
// sent. Drafting spends the account's own AI key, and that stays a decision a
// person makes.
//
// @Summary     Hunt the territory from the catalogue, no words needed
// @Description Reads the account's products, resolves them against the CNES dictionary and creates leads for the biggest establishments in the territory it has never approached. Capped per run. Drafts nothing and sends nothing.
// @Tags        Me
// @Produce     json
// @Success     200 {object} autoHuntResponse
// @Failure     401 {object} domain.AppError
// @Failure     503 {object} domain.AppError
// @Security    BearerAuth
// @Router      /me/territory/auto-hunt [post]
func (h *MeHandler) HandleAutoHunt(c *gin.Context) {
	if h.autoHuntUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "RADAR_UNAVAILABLE", Message: "radar not configured"})
		return
	}
	res, err := h.autoHuntUC.RunForAccount(c.Request.Context(), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, autoHuntResponse{Found: res.Found, Created: res.Created})
}
