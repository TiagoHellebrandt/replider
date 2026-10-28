package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/usecases"
)

// TwoFactorHandler exposes the opt-in TOTP enrollment flow under
// /v1/me/2fa. The owner-panel enforcement (phase 2) layers on top:
// these endpoints work for any authenticated user today, and only
// the super-admin login flow will require a successful Verify in a
// later commit.
type TwoFactorHandler struct {
	uc *usecases.TwoFactorUseCase
}

func NewTwoFactorHandler(uc *usecases.TwoFactorUseCase) *TwoFactorHandler {
	return &TwoFactorHandler{uc: uc}
}

// HandleStatus reports whether 2FA is enabled for the current user.
// The frontend reads this to decide whether to render the "Set up
// 2FA" CTA or the "Disable 2FA" management card.
func (h *TwoFactorHandler) HandleStatus(c *gin.Context) {
	enabled, err := h.uc.Status(c.Request.Context(), c.GetString("userID"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"enabled": enabled})
}

// HandleSetup stages a fresh TOTP secret and returns it + the
// otpauth URL. Idempotent on retry: calling Setup twice rotates
// the staged secret so an interrupted enrollment can restart cleanly.
func (h *TwoFactorHandler) HandleSetup(c *gin.Context) {
	userID := c.GetString("userID")
	email := c.GetString("userEmail")
	out, err := h.uc.Setup(c.Request.Context(), userID, email)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type verifyTwoFactorReq struct {
	Code string `json:"code" binding:"required"`
}

// HandleVerify validates the staged secret with a 6-digit code,
// flips totp_enabled to true, and returns plaintext recovery codes
// (shown to the user exactly once).
func (h *TwoFactorHandler) HandleVerify(c *gin.Context) {
	var req verifyTwoFactorReq
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.uc.Verify(c.Request.Context(), c.GetString("userID"), req.Code)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandleReverify re-proves possession of the second factor for an
// ALREADY authenticated session, refreshing totp_verified_at.
//
// This is the step-up half of the owner-panel gate: RequireSuperAdmin
// refuses with OWNER_2FA_STALE once the last proof is older than
// SuperAdminFreshness (12h), and without this endpoint the only way to
// refresh it was a full logout + login — which threw away whatever the
// operator had half-typed. Same strength as the login pivot: a valid JWT
// is NOT enough, a current TOTP (or an unused recovery) code is required,
// and it is rate-limited like the login verify.
//
// Deliberately NOT HandleVerify: that one is the enrollment confirm and
// rotates the recovery codes on every call.
func (h *TwoFactorHandler) HandleReverify(c *gin.Context) {
	var req verifyTwoFactorReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.uc.VerifyForLogin(c.Request.Context(), c.GetString("userID"), req.Code); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type disableTwoFactorReq struct {
	Code string `json:"code" binding:"required"`
}

// HandleDisable tears down 2FA. Requires a current TOTP code or a
// recovery code in the body, so a leaked auth JWT alone cannot
// disable the second factor.
func (h *TwoFactorHandler) HandleDisable(c *gin.Context) {
	var req disableTwoFactorReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.uc.Disable(c.Request.Context(), c.GetString("userID"), req.Code); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
