package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Target audiences (§2, D3).
type AudienceHandler struct {
	uc *usecases.AudienceUseCase
}

func NewAudienceHandler(uc *usecases.AudienceUseCase) *AudienceHandler {
	return &AudienceHandler{uc: uc}
}

// HandleList serves GET /v1/audiences.
//
// @Summary  List target audiences
// @Tags     audiences
// @Produce  json
// @Success  200 {array} domain.Audience
// @Router   /audiences [get]
func (h *AudienceHandler) HandleList(c *gin.Context) {
	list, err := h.uc.List(c.Request.Context(), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

type audienceRequest struct {
	Name          string   `json:"name"`
	Segment       string   `json:"segment"`
	Size          string   `json:"size"`
	DecisionMaker string   `json:"decision_maker"`
	Regions       []string `json:"regions"`
	Keywords      []string `json:"keywords"`
}

// HandleUpsert serves PUT /v1/audiences/:id ("new" creates).
//
// @Summary  Create or update a target audience
// @Tags     audiences
// @Accept   json
// @Produce  json
// @Param    id path string true "Audience ID, or 'new'"
// @Success  200 {object} domain.Audience
// @Failure  400 {object} domain.AppError
// @Router   /audiences/{id} [put]
func (h *AudienceHandler) HandleUpsert(c *gin.Context) {
	var req audienceRequest
	if !bindJSON(c, &req) {
		return
	}
	id := c.Param("id")
	if id == "new" {
		id = ""
	}
	saved, err := h.uc.Save(c.Request.Context(), getAccountID(c), &domain.Audience{
		ID: id, Name: req.Name, Segment: req.Segment, Size: req.Size,
		DecisionMaker: req.DecisionMaker, Regions: req.Regions,
		Keywords: req.Keywords,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, saved)
}

// HandleDelete serves DELETE /v1/audiences/:id.
//
// @Summary  Delete a target audience
// @Tags     audiences
// @Param    id path string true "Audience ID"
// @Success  204
// @Router   /audiences/{id} [delete]
func (h *AudienceHandler) HandleDelete(c *gin.Context) {
	if err := h.uc.Delete(c.Request.Context(), c.Param("id"), getAccountID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleSuggest serves POST /v1/audiences/suggest.
//
// Returns proposals WITHOUT saving them. A suggestion written straight into
// somebody's list would be the product deciding who they sell to.
//
// @Summary  Suggest target audiences
// @Tags     audiences
// @Produce  json
// @Success  200 {array} domain.Audience
// @Failure  400 {object} domain.AppError
// @Router   /audiences/suggest [post]
func (h *AudienceHandler) HandleSuggest(c *gin.Context) {
	list, err := h.uc.Suggest(c.Request.Context(), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	if list == nil {
		list = []*domain.Audience{}
	}
	c.JSON(http.StatusOK, list)
}
