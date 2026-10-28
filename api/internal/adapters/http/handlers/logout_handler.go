package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// LogoutHandler revokes the current JWT token so it can no longer be
// used for authentication, even before its natural expiration.
type LogoutHandler struct {
	blacklist  domain.TokenBlacklist
	jwtService domain.JWTService
}

func NewLogoutHandler(blacklist domain.TokenBlacklist, jwtService domain.JWTService) *LogoutHandler {
	return &LogoutHandler{blacklist: blacklist, jwtService: jwtService}
}

// HandleLogout godoc
// @Summary      Logout (revoke token)
// @Description  Revokes the current JWT token. The token is added to a
//
//	blacklist and will be rejected by the auth middleware on
//	subsequent requests, even if it hasn't expired yet.
//
// @Tags         auth
// @Security     BearerAuth
// @Success      200  {object}  map[string]string "Token revoked"
// @Failure      401  {object}  domain.AppError "Unauthorized"
// @Router       /auth/logout [post]
func (h *LogoutHandler) HandleLogout(c *gin.Context) {
	token := c.GetString("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code: "INVALID_REQUEST", Message: "no token in context",
		})
		return
	}

	// Parse the token to get its expiration time, so we can set
	// the blacklist TTL to match (no point blacklisting beyond expiry).
	claims, err := h.jwtService.ValidateToken(token)
	if err != nil {
		// Token is already invalid — nothing to revoke.
		c.JSON(http.StatusOK, gin.H{"message": "token already expired"})
		return
	}

	remaining := time.Until(claims.ExpiresAt.Time)
	if remaining <= 0 {
		c.JSON(http.StatusOK, gin.H{"message": "token already expired"})
		return
	}

	if err := h.blacklist.Revoke(c.Request.Context(), token, remaining); err != nil {
		// Non-fatal: the token will expire naturally anyway.
		c.JSON(http.StatusOK, gin.H{"message": "token revoked (best-effort)"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "token revoked"})
}
