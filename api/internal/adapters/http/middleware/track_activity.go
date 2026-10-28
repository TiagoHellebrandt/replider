package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// TrackActivity updates users.last_seen_at so the Team screen can tell
// admins which members are actively using the workspace. Runs AFTER
// AuthMiddleware (depends on the userID being in context) and before
// the handler. The SQL layer throttles writes to once per 5 minutes so
// this is cheap to call on every authenticated request.
//
// Failure is never fatal — if the DB is unhappy we log and move on;
// a broken activity tracker should not break the whole API.
//
// Intentionally NOT calling c.Next(): TrackActivity is invoked INLINE
// inside the authMw composite in router.Setup (rawAuthMw + trackMw,
// then authPaidMw runs paidMw afterwards). If trackMw called c.Next()
// it would run the entire remaining gin chain (RequireRole + the
// handler) inline before returning to authPaidMw — so the handler
// would write its 200 + body and the subscription check would then
// abort with 402, appending TRIAL_EXPIRED to the same response. The
// client got `[{...agents...}]{"code":"TRIAL_EXPIRED"}` and crashed
// the JSON parser at the boundary. Returning without c.Next() lets
// gin's outer dispatch advance normally when this is the last
// inline-composed middleware on the auth path.
func TrackActivity(userRepo domain.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		if userID == "" {
			return
		}
		if err := userRepo.UpdateLastSeen(c.Request.Context(), userID); err != nil {
			slog.Warn("track activity: update last_seen_at failed",
				"user_id", userID, "error", err)
		}
	}
}
