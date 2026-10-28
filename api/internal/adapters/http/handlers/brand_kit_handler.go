package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// BrandKitHandler exposes the company voice layer and the seller's overlay.
//
// Two different permission shapes on purpose: the company layer defines what
// the business may claim, which is an admin decision, while the overlay is
// how one person signs their own messages.
type BrandKitHandler struct {
	repo domain.BrandKitRepository
	// objections is the account's pushback library (objeções v1), wired via
	// SetObjectionRepo — see objection_handler.go.
	objections domain.ObjectionRepository
	// differentiators is the "why ours" library — see differentiator_handler.go.
	differentiators domain.DifferentiatorRepository
}

func NewBrandKitHandler(repo domain.BrandKitRepository) *BrandKitHandler {
	return &BrandKitHandler{repo: repo}
}

// HandleGet serves GET /v1/brand-kit.
//
// Returns an EMPTY kit rather than 404 when none is configured: the screen
// that reads this is a form, and a form that has to special-case "not created
// yet" gets that case wrong.
//
// @Summary      Read the company brand kit
// @Tags         brand
// @Produce      json
// @Success      200 {object} domain.BrandKit
// @Security     BearerAuth
// @Router       /brand-kit [get]
func (h *BrandKitHandler) HandleGet(c *gin.Context) {
	accountID := getAccountID(c)
	kit, err := h.repo.Get(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, err)
		return
	}
	if kit == nil {
		kit = &domain.BrandKit{
			AccountID:       accountID,
			AllowedClaims:   []string{},
			ForbiddenClaims: []string{},
		}
	}
	c.JSON(http.StatusOK, kit)
}

type brandKitRequest struct {
	CompanyName     string   `json:"company_name"`
	About           string   `json:"about"`
	ToneOfVoice     string   `json:"tone_of_voice"`
	LogoURL         string   `json:"logo_url"`
	PrimaryColor    string   `json:"primary_color"`
	Signature       string   `json:"signature"`
	AllowedClaims   []string `json:"allowed_claims"`
	ForbiddenClaims []string `json:"forbidden_claims"`
}

// HandleSave serves PUT /v1/brand-kit. Admin only — this decides what the
// business may claim, which is not one salesperson's call.
//
// @Summary      Update the company brand kit
// @Tags         brand
// @Accept       json
// @Produce      json
// @Param        request body brandKitRequest true "Brand kit"
// @Success      200 {object} domain.BrandKit
// @Security     BearerAuth
// @Router       /brand-kit [put]
func (h *BrandKitHandler) HandleSave(c *gin.Context) {
	var req brandKitRequest
	if !bindJSON(c, &req) {
		return
	}
	kit := &domain.BrandKit{
		AccountID:       getAccountID(c),
		CompanyName:     req.CompanyName,
		About:           req.About,
		ToneOfVoice:     req.ToneOfVoice,
		LogoURL:         req.LogoURL,
		PrimaryColor:    req.PrimaryColor,
		Signature:       req.Signature,
		AllowedClaims:   trimAll(req.AllowedClaims),
		ForbiddenClaims: trimAll(req.ForbiddenClaims),
	}
	if err := h.repo.Save(c.Request.Context(), kit); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, kit)
}

type sellerOverlayRequest struct {
	ToneNote  string `json:"tone_note"`
	Signature string `json:"signature"`
}

// HandleGetOverlay serves GET /v1/me/seller-overlay.
//
// @Summary      Read your personal voice overlay
// @Tags         brand
// @Produce      json
// @Success      200 {object} domain.SellerOverlay
// @Security     BearerAuth
// @Router       /me/seller-overlay [get]
func (h *BrandKitHandler) HandleGetOverlay(c *gin.Context) {
	actor := actorFrom(c)
	o, err := h.repo.GetOverlay(c.Request.Context(), actor.AccountID, actor.UserID)
	if err != nil {
		respondError(c, err)
		return
	}
	if o == nil {
		o = &domain.SellerOverlay{AccountID: actor.AccountID, UserID: actor.UserID}
	}
	c.JSON(http.StatusOK, o)
}

// HandleSaveOverlay serves PUT /v1/me/seller-overlay.
//
// Scoped to the caller, always: the user id comes from the token and never
// from the body, so this cannot be used to rewrite a colleague's signature.
//
// @Summary      Update your personal voice overlay
// @Tags         brand
// @Accept       json
// @Produce      json
// @Param        request body sellerOverlayRequest true "Overlay"
// @Success      200 {object} domain.SellerOverlay
// @Security     BearerAuth
// @Router       /me/seller-overlay [put]
func (h *BrandKitHandler) HandleSaveOverlay(c *gin.Context) {
	var req sellerOverlayRequest
	if !bindJSON(c, &req) {
		return
	}
	actor := actorFrom(c)
	o := &domain.SellerOverlay{
		AccountID: actor.AccountID,
		UserID:    actor.UserID,
		ToneNote:  req.ToneNote,
		Signature: req.Signature,
	}
	if err := h.repo.SaveOverlay(c.Request.Context(), o); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, o)
}

// trimAll drops blank entries: a form with an empty row must not persist a
// forbidden claim of "", which would match every text ever written.
func trimAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}
