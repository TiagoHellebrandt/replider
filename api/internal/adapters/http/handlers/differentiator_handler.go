package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
)

// The "why ours" library, edited the same way the objection library is: read
// by anyone on the floor (a seller needs to know the argument), written by an
// admin (it is what the company commits to saying).

// SetDifferentiatorRepo wires the library.
func (h *BrandKitHandler) SetDifferentiatorRepo(r domain.DifferentiatorRepository) {
	h.differentiators = r
}

type differentiatorsResponse struct {
	Items []domain.Differentiator `json:"items"`
}

type differentiatorsRequest struct {
	Items []domain.Differentiator `json:"items"`
}

// maxDifferentiators bounds the library. A list nobody can hold in their head
// is a list the model picks from at random.
const maxDifferentiators = 30

// HandleListDifferentiators serves GET /v1/differentiators.
//
// An empty list comes back empty. Unlike objections there is deliberately no
// seed: a default objection is a fact about the market, a default
// differentiator would be the product inventing an advantage about somebody
// else's equipment.
//
// @Summary     The account's differentiators
// @Tags        Brand
// @Produce     json
// @Success     200 {object} differentiatorsResponse
// @Security    BearerAuth
// @Router      /differentiators [get]
func (h *BrandKitHandler) HandleListDifferentiators(c *gin.Context) {
	if h.differentiators == nil {
		respondError(c, &domain.AppError{
			Code: "DIFFERENTIATORS_UNAVAILABLE", Message: "differentiator library not configured"})
		return
	}
	rows, err := h.differentiators.List(getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	if rows == nil {
		rows = []domain.Differentiator{}
	}
	c.JSON(http.StatusOK, differentiatorsResponse{Items: rows})
}

// HandleSaveDifferentiators serves PUT /v1/differentiators (admin).
//
// @Summary     Replace the account's differentiators
// @Tags        Brand
// @Accept      json
// @Produce     json
// @Param       payload body differentiatorsRequest true "The whole list, in order"
// @Success     200 {object} differentiatorsResponse
// @Failure     400 {object} domain.AppError
// @Security    BearerAuth
// @Router      /differentiators [put]
func (h *BrandKitHandler) HandleSaveDifferentiators(c *gin.Context) {
	if h.differentiators == nil {
		respondError(c, &domain.AppError{
			Code: "DIFFERENTIATORS_UNAVAILABLE", Message: "differentiator library not configured"})
		return
	}
	var req differentiatorsRequest
	if !bindJSON(c, &req) {
		return
	}
	accountID := getAccountID(c)

	clean := make([]domain.Differentiator, 0, len(req.Items))
	for _, it := range req.Items {
		it.Label = strings.TrimSpace(it.Label)
		it.Detail = strings.TrimSpace(it.Detail)
		// A blank row is somebody's unfinished thought, not a differentiator.
		if it.Label == "" {
			continue
		}
		it.AccountID = accountID
		clean = append(clean, it)
		if len(clean) >= maxDifferentiators {
			break
		}
	}
	if err := h.differentiators.Replace(accountID, clean); err != nil {
		respondError(c, err)
		return
	}
	rows, err := h.differentiators.List(accountID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, differentiatorsResponse{Items: rows})
}
