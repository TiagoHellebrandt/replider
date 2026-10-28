package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Owner panel Vendedores section (owner-sellers US-001): list the
// platform account's users with full seller state + atomic onboard.

// SetOwnerSellersUC wires the Vendedores endpoints (same optional-setter
// pattern as SetCommissionUC). nil → those routes 503.
func (h *OwnerHandler) SetOwnerSellersUC(uc *usecases.OwnerSellersUseCase) {
	h.ownerSellersUC = uc
}

func (h *OwnerHandler) ownerSellersUCOrAbort(c *gin.Context) *usecases.OwnerSellersUseCase {
	if h.ownerSellersUC == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "sellers admin is not configured"})
		return nil
	}
	return h.ownerSellersUC
}

// @Summary      List the platform account's users with seller state (flag, activity, ref code, PIX, rotation, carteira)
// @Tags         owner
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Security     BearerAuth
// @Router       /owner/sellers [get]
func (h *OwnerHandler) HandleListSellers(c *gin.Context) {
	uc := h.ownerSellersUCOrAbort(c)
	if uc == nil {
		return
	}
	rows, err := uc.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

type onboardSellerReq struct {
	IsSeller *bool `json:"is_seller" binding:"required"`
}

// @Summary      Atomically onboard (or offboard) a platform user as seller: flag + ref code + rotation in one transaction
// @Tags         owner
// @Accept       json
// @Produce      json
// @Param        id path string true "Platform user ID"
// @Param        body body onboardSellerReq true "Seller state"
// @Success      200 {object} domain.OwnerSellerRow
// @Security     BearerAuth
// @Router       /owner/sellers/{id}/onboard [put]
func (h *OwnerHandler) HandleOnboardSeller(c *gin.Context) {
	uc := h.ownerSellersUCOrAbort(c)
	if uc == nil {
		return
	}
	var req onboardSellerReq
	if err := c.ShouldBindJSON(&req); err != nil || req.IsSeller == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "is_seller (bool) is required"})
		return
	}
	var row *domain.OwnerSellerRow
	row, err := uc.SetSeller(c.Request.Context(), c.GetString("userEmail"), c.Param("id"), *req.IsSeller)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, row)
}
