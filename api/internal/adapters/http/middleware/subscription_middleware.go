package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// RequireActiveSubscription guards paid routes. It blocks requests when the
// account's trial window has elapsed and no paid subscription has taken over,
// or when Stripe has reported the subscription as past_due/canceled.
//
// Must run AFTER AuthMiddleware (depends on accountID in context). Returns
// 402 Payment Required with a stable error code the app can intercept to
// redirect the user to /billing.
//
// Read-only requests (GET/HEAD/OPTIONS) bypass the check. The product
// decision is that a trial-expired user keeps READ access to their
// agents/channels/knowledge — the data is theirs, seeing it preserves
// continuity, and the "look but can't touch" view is a stronger
// motivator to upgrade than a flat redirect to /billing. Writes
// (POST/PUT/PATCH/DELETE) still 402 so they can't burn AI tokens or
// modify state without an active subscription.
func RequireActiveSubscription(accountRepo domain.AccountRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Refuse to answer twice. A gate that writes after the handler has
		// already responded does not block anything — it corrupts the body
		// instead, concatenating two JSON documents under one 200. A client
		// decoding that gets a parse error, not a permission error, so the
		// failure surfaces as "the app is broken" rather than "you cannot
		// do this". Whatever ordering bug puts us here, appending is never
		// the right answer.
		deny := func(status int, body gin.H) {
			if c.Writer.Written() {
				slog.ErrorContext(c.Request.Context(),
					"subscription gate reached after the response was written; refusing to append",
					"path", c.FullPath(), "code", body["code"])
				c.Abort()
				return
			}
			c.AbortWithStatusJSON(status, body)
		}

		// Reads pass through. Anything that mutates state or burns
		// provider tokens (POST/PUT/PATCH/DELETE) goes through the
		// subscription check below. We allow OPTIONS for CORS preflights
		// — those never reach a handler that mutates state, gin's CORS
		// middleware short-circuits them.
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			return
		}

		accountID := c.GetString("accountID")
		if accountID == "" {
			// No account in context — let the auth layer handle it. Plain
			// `return` on purpose: gin auto-advances when a middleware
			// returns without aborting, and an inline c.Next() here would
			// run the whole chain (handler included) BEFORE returning to
			// the composite caller — the exact double-write shape the
			// comment further down describes.
			return
		}

		account, err := accountRepo.GetByID(c.Request.Context(), accountID)
		if err != nil || account == nil {
			deny(http.StatusUnauthorized, gin.H{
				"code": "UNAUTHORIZED", "message": "account not found",
			})
			return
		}

		// Owner-panel suspension takes precedence over every other
		// gate. Reads still pass (the early-return above already
		// allowed GET/HEAD/OPTIONS) — writes are blocked with a stable
		// error code so the dashboard can surface the suspended-banner
		// without guessing.
		if account.IsSuspended() {
			reason := ""
			if account.SuspendedReason != nil {
				reason = *account.SuspendedReason
			}
			deny(http.StatusForbidden, gin.H{
				"code":    "ACCOUNT_SUSPENDED",
				"message": "this account has been suspended by the platform",
				"reason":  reason,
			})
			return
		}

		// Trial bypass: owner self-test accounts + Meta App Reviewer
		// accounts skip the trial-expiration gate entirely. Plan
		// limits still apply via the plan_limit middleware further
		// down the chain, so a bypassed account can't burn through
		// unlimited AI tokens — only the trial-expiry check is
		// short-circuited.
		if account.TrialBypass {
			return
		}

		// We intentionally do NOT call c.Next() on the success paths:
		// gin auto-advances to the next handler when a middleware
		// returns without aborting. Calling c.Next() inline would run
		// the WHOLE chain (including the request handler) BEFORE
		// returning to the composite authPaidMw caller, which then
		// produced double-write responses (handler wrote 200 + body,
		// then this middleware wrote 402 TRIAL_EXPIRED on top).
		switch account.SubscriptionStatus {
		case domain.SubscriptionStatusActive:
			return
		case domain.SubscriptionStatusTrialing:
			if account.IsTrialExpired(time.Now()) {
				deny(http.StatusPaymentRequired, gin.H{
					"code":    "TRIAL_EXPIRED",
					"message": "your free trial has ended — upgrade to continue",
				})
				return
			}
			return
		default:
			// past_due, canceled, or unknown — require a successful invoice
			// before restoring access.
			deny(http.StatusPaymentRequired, gin.H{
				"code":    "SUBSCRIPTION_INACTIVE",
				"message": "your subscription is not active — please update your payment method",
			})
		}
	}
}
