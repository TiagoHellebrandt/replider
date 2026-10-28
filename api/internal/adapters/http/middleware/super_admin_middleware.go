package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// SuperAdminFreshness gates how recent a 2FA proof must be for the
// owner-panel session to remain valid. The login pivot stamps
// totp_verified_at on the verify-login round-trip; once that timestamp
// is older than this window, the middleware returns 401 OWNER_2FA_STALE
// and the frontend should re-prompt the operator with a TOTP code.
const SuperAdminFreshness = 12 * time.Hour

// RequireSuperAdmin gates a route group to operators on the platform-
// level allowlist. Email lookup happens against the user repo because
// the JWT does NOT carry email today (only userID/role). The allowlist
// is sourced from the SUPER_ADMIN_EMAILS env var (comma-separated,
// lowercased, trimmed) — keeps the config out of the DB so no super-
// admin can grant themselves to peers without a deploy.
//
// When require2FA is true, the middleware additionally requires the
// allowlisted user to have totp_enabled = true AND a totp_verified_at
// fresher than SuperAdminFreshness. The flag is fed from the
// SUPER_ADMIN_REQUIRE_2FA env (parsed in cfg) so a fresh deploy can
// stage 2FA enrollment before flipping the gate without an extra ship.
//
// On success the handler sets `isSuperAdmin=true` + `userEmail` in the
// gin context so downstream handlers can read the operator's email
// without re-fetching the user row.
func RequireSuperAdmin(userRepo domain.UserRepository, allowlist []string, require2FA bool) gin.HandlerFunc {
	// Pre-normalize the allowlist once at construction.
	normalized := make([]string, 0, len(allowlist))
	for _, e := range allowlist {
		v := strings.ToLower(strings.TrimSpace(e))
		if v != "" {
			normalized = append(normalized, v)
		}
	}
	return func(c *gin.Context) {
		if len(normalized) == 0 {
			// Fail closed when allowlist is empty — never want a deploy
			// missing the env to silently expose owner endpoints.
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "SUPER_ADMIN_NOT_CONFIGURED",
				"message": "super admin allowlist is empty",
			})
			return
		}
		userID := c.GetString("userID")
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "UNAUTHORIZED",
				"message": "authentication required",
			})
			return
		}
		user, err := userRepo.GetByID(c.Request.Context(), userID)
		if err != nil || user == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "FORBIDDEN",
				"message": "user not found",
			})
			return
		}
		email := strings.ToLower(strings.TrimSpace(user.Email))
		allowed := false
		for _, a := range normalized {
			if email == a {
				allowed = true
				break
			}
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "FORBIDDEN",
				"message": "super admin only",
			})
			return
		}
		if require2FA {
			if !user.TOTPEnabled {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"code":    "OWNER_2FA_REQUIRED",
					"message": "Enroll a TOTP authenticator before accessing the owner panel",
				})
				return
			}
			if user.TOTPVerifiedAt == nil || time.Since(*user.TOTPVerifiedAt) > SuperAdminFreshness {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"code":    "OWNER_2FA_STALE",
					"message": "Re-enter your 2FA code to continue",
				})
				return
			}
		}
		c.Set("isSuperAdmin", true)
		c.Set("userEmail", user.Email)
		c.Next()
	}
}
