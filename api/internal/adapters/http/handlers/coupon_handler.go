package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/usecases"
)

// CouponHandler exposes the owner-panel CRUD over /v1/owner/coupons
// plus the public lookup at /v1/coupons/:code. The owner routes are
// already gated by RequireSuperAdmin upstream.
type CouponHandler struct {
	uc *usecases.CouponAdminUseCase
}

func NewCouponHandler(uc *usecases.CouponAdminUseCase) *CouponHandler {
	return &CouponHandler{uc: uc}
}

type createCouponReq struct {
	Code           string  `json:"code" binding:"required"`
	PercentOff     int     `json:"percent_off" binding:"required"`
	MaxRedemptions *int    `json:"max_redemptions"`
	ExpiresAt      *string `json:"expires_at"` // RFC3339
}

func (h *CouponHandler) HandleCreate(c *gin.Context) {
	var req createCouponReq
	if !bindJSON(c, &req) {
		return
	}
	dto := usecases.CreateCouponDTO{
		ActorEmail:     c.GetString("userEmail"),
		Code:           req.Code,
		PercentOff:     req.PercentOff,
		MaxRedemptions: req.MaxRedemptions,
	}
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expires_at, expected RFC3339"})
			return
		}
		dto.ExpiresAt = &t
	}
	out, err := h.uc.Create(c.Request.Context(), dto)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *CouponHandler) HandleList(c *gin.Context) {
	limit, offset := pagination(c)
	rows, err := h.uc.List(c.Request.Context(), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (h *CouponHandler) HandleDeactivate(c *gin.Context) {
	if err := h.uc.Deactivate(c.Request.Context(), c.GetString("userEmail"), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// HandlePublicLookup is unauthenticated — the register / billing
// page calls it to render the discount preview. Returns
// {valid: false} on miss instead of 404 to keep the response shape
// stable for the front-end.
func (h *CouponHandler) HandlePublicLookup(c *gin.Context) {
	out := h.uc.LookupForPublic(c.Request.Context(), c.Param("code"))
	c.JSON(http.StatusOK, out)
}
