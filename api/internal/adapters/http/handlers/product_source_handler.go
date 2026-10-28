package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// SetSourceLinkUC wires the brochure door. Optional setter so neither
// NewProductHandler nor router.Setup changes signature — nil answers 404
// through the use case's own "this product has no archived brochure".
func (h *ProductHandler) SetSourceLinkUC(uc *usecases.ProductSourceLinkUseCase) {
	h.sourceLinkUC = uc
}

// HandleGetSource returns the brochure a product was read from: the filename a
// person recognises, the page the model read it on, and a short-lived URL.
//
// @Summary     Brochure a product was imported from
// @Tags        products
// @Produce     json
// @Param       id path string true "Product ID"
// @Success     200 {object} usecases.ProductSourceLink
// @Failure     404 {object} domain.AppError
// @Router      /products/{id}/source [get]
func (h *ProductHandler) HandleGetSource(c *gin.Context) {
	if h.sourceLinkUC == nil {
		respondError(c, usecases.ErrProductSourceNotFound)
		return
	}
	link, err := h.sourceLinkUC.Execute(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, link)
}

// HandleGetImages returns the brochure pictures for a product's page, each
// with a short-lived signed URL.
//
// @Summary     Brochure images for a product
// @Tags        products
// @Produce     json
// @Param       id path string true "Product ID"
// @Success     200 {array} usecases.ProductImage
// @Failure     404 {object} domain.AppError
// @Router      /products/{id}/images [get]
func (h *ProductHandler) HandleGetImages(c *gin.Context) {
	if h.sourceLinkUC == nil {
		respondError(c, usecases.ErrProductSourceNotFound)
		return
	}
	images, err := h.sourceLinkUC.Images(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, images)
}

var _ = domain.ErrProductNotFound // swagger needs the domain import in code
