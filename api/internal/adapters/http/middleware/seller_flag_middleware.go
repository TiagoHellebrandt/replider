package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SellerFlagChecker reports whether a user carries the explicit
// owner-granted seller flag (seller_settings.is_seller, US-014).
// Satisfied by postgres.SellerRefRepository.
type SellerFlagChecker interface {
	IsSeller(ctx context.Context, userID string) (bool, error)
}

// RequireSeller gates the /seller panel to users the owner explicitly
// flagged as sellers (US-014) — being on the platform account is not
// enough, otherwise every attendant silently becomes a payable seller
// the moment they open the panel. Runs AFTER auth + RequirePlatformAccount
// (relies on the "userID" context key).
//
// 403 SELLER_NOT_FLAGGED for unflagged users (the panel's existence is
// already known inside the platform account — no need for the 404
// treatment RequirePlatformAccount uses). Fail-closed: a checker error
// also denies (503), never lets an unverified user through.
func RequireSeller(checker SellerFlagChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		if checker == nil || userID == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code": "SELLER_NOT_FLAGGED", "message": "user is not a registered seller"})
			return
		}
		isSeller, err := checker.IsSeller(c.Request.Context(), userID)
		if err != nil {
			slog.Error("seller gate: flag lookup failed — denying (fail-closed)",
				"user", userID, "error", err)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"code": "SELLER_GATE_UNAVAILABLE", "message": "could not verify seller access"})
			return
		}
		if !isSeller {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code": "SELLER_NOT_FLAGGED", "message": "user is not a registered seller — ask the owner to grant seller access"})
			return
		}
		c.Next()
	}
}
