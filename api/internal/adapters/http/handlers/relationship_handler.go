package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Relationship routines and cross-sell (§6.5, F4/F5).
//
// Two features, one handler: both answer "what should I say to a customer I
// already have", and both are read by the same screen.
type RelationshipHandler struct {
	uc *usecases.RelationshipUseCase
	// crossSell is optional. Without it the dates still work — an account
	// with no sales has no co-purchase patterns to show anyway.
	crossSell *usecases.CrossSellUseCase
}

func NewRelationshipHandler(uc *usecases.RelationshipUseCase) *RelationshipHandler {
	return &RelationshipHandler{uc: uc}
}

// SetCrossSell wires the co-purchase suggestions.
func (h *RelationshipHandler) SetCrossSell(uc *usecases.CrossSellUseCase) { h.crossSell = uc }

// HandleList serves GET /v1/relationship-dates.
//
// @Summary  Dates worth remembering, soonest first
// @Tags     relationship
// @Produce  json
// @Success  200 {array} domain.RelationshipDate
// @Router   /relationship-dates [get]
func (h *RelationshipHandler) HandleList(c *gin.Context) {
	dates, err := h.uc.List(c.Request.Context(), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, dates)
}

// HandleSave serves POST /v1/relationship-dates.
//
// @Summary  Create or update a date
// @Tags     relationship
// @Accept   json
// @Produce  json
// @Success  200 {object} domain.RelationshipDate
// @Failure  400 {object} domain.AppError
// @Router   /relationship-dates [post]
func (h *RelationshipHandler) HandleSave(c *gin.Context) {
	var req usecases.RelationshipDateInput
	if !bindJSON(c, &req) {
		return
	}
	date, err := h.uc.Save(c.Request.Context(), getAccountID(c), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, date)
}

// HandleDelete serves DELETE /v1/relationship-dates/:id.
//
// @Summary  Forget a date
// @Tags     relationship
// @Param    id path string true "Date ID"
// @Success  204
// @Router   /relationship-dates/{id} [delete]
func (h *RelationshipHandler) HandleDelete(c *gin.Context) {
	if err := h.uc.Delete(c.Request.Context(), getAccountID(c), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleSectors serves GET /v1/relationship-dates/sectors.
//
// The shipped catalogue of Brazilian professional days. Static, but served
// rather than duplicated in the client so the two can never disagree about
// when Dia do Médico is.
//
// @Summary  Professional days an account can subscribe to
// @Tags     relationship
// @Produce  json
// @Success  200 {array} domain.SectorDate
// @Router   /relationship-dates/sectors [get]
func (h *RelationshipHandler) HandleSectors(c *gin.Context) {
	c.JSON(http.StatusOK, h.uc.Sectors())
}

// HandleCrossSell serves GET /v1/portfolio/:id/cross-sell.
//
// @Summary  What else to offer this customer
// @Tags     relationship
// @Produce  json
// @Param    id path string true "Contact ID"
// @Success  200 {object} usecases.CrossSellResult
// @Failure  503 {object} domain.AppError
// @Router   /portfolio/{id}/cross-sell [get]
func (h *RelationshipHandler) HandleCrossSell(c *gin.Context) {
	if h.crossSell == nil {
		respondError(c, &domain.AppError{
			Code: "CROSS_SELL_UNAVAILABLE", Message: "sugestões de venda casada não configuradas"})
		return
	}
	limit, _ := pagination(c)
	if limit > 10 {
		// A panel is not a catalogue. Ten suggestions is already more than
		// anybody reads before a call.
		limit = 10
	}
	result, err := h.crossSell.Suggest(c.Request.Context(), getAccountID(c), c.Param("id"), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// HandleCrossSellPatterns serves GET /v1/cross-sell/patterns.
//
// What the account has learned from its own sales, so a suggestion can be
// explained rather than trusted.
//
// @Summary  Co-purchase patterns learned from this account's sales
// @Tags     relationship
// @Produce  json
// @Success  200 {array} domain.CoPurchasePair
// @Failure  503 {object} domain.AppError
// @Router   /cross-sell/patterns [get]
func (h *RelationshipHandler) HandleCrossSellPatterns(c *gin.Context) {
	if h.crossSell == nil {
		respondError(c, &domain.AppError{
			Code: "CROSS_SELL_UNAVAILABLE", Message: "sugestões de venda casada não configuradas"})
		return
	}
	pairs, err := h.crossSell.Patterns(c.Request.Context(), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, pairs)
}
