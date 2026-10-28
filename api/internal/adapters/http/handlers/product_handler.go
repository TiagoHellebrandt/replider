package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/usecases"
)

// ProductHandler exposes the catalog product CRUD (US-004). All routes are
// mounted under admin + paid middleware; account scoping is enforced by the
// usecase via the accountID claim.
type ProductHandler struct {
	uc *usecases.ProductUseCase
	// sourceLinkUC opens the brochure a product was imported from (optional).
	sourceLinkUC *usecases.ProductSourceLinkUseCase
}

func NewProductHandler(uc *usecases.ProductUseCase) *ProductHandler {
	return &ProductHandler{uc: uc}
}

type productReq struct {
	Name        string          `json:"name" binding:"required"`
	Description string          `json:"description"`
	PriceCents  int64           `json:"price_cents"`
	Currency    string          `json:"currency"`
	Attributes  json.RawMessage `json:"attributes"`
	Active      *bool           `json:"active"`
}

func (h *ProductHandler) HandleCreate(c *gin.Context) {
	accountID := getAccountID(c)
	var req productReq
	if !bindJSON(c, &req) {
		return
	}

	p, err := h.uc.Create(c.Request.Context(), usecases.CreateProductDTO{
		AccountID:   accountID,
		Name:        req.Name,
		Description: req.Description,
		PriceCents:  req.PriceCents,
		Currency:    req.Currency,
		Attributes:  req.Attributes,
		Active:      req.Active,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *ProductHandler) HandleList(c *gin.Context) {
	accountID := getAccountID(c)
	products, err := h.uc.List(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, products)
}

func (h *ProductHandler) HandleGet(c *gin.Context) {
	accountID := getAccountID(c)
	p, err := h.uc.Get(c.Request.Context(), accountID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *ProductHandler) HandleUpdate(c *gin.Context) {
	accountID := getAccountID(c)
	var req productReq
	if !bindJSON(c, &req) {
		return
	}

	p, err := h.uc.Update(c.Request.Context(), usecases.UpdateProductDTO{
		AccountID:   accountID,
		ID:          c.Param("id"),
		Name:        req.Name,
		Description: req.Description,
		PriceCents:  req.PriceCents,
		Currency:    req.Currency,
		Attributes:  req.Attributes,
		Active:      req.Active,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *ProductHandler) HandleDelete(c *gin.Context) {
	accountID := getAccountID(c)
	if err := h.uc.Delete(c.Request.Context(), accountID, c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "product deleted"})
}
