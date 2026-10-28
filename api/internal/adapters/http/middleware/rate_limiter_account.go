package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	limiter "github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// RateLimiterByAccount returns a middleware that counts requests per
// account_id (set by AuthMiddleware) instead of per client IP.
//
// Why this exists: IP limits don't defend against the common SaaS abuse
// case where a single paying account hammers AI endpoints — either via a
// buggy integration, a compromised API key, or someone testing a script.
// Without per-account caps, one customer can drain the shared provider
// quota and we pay the overage.
//
// Falls back to the client IP when the request is not authenticated
// (public endpoints), so the middleware composes cleanly in any route
// group.
func RateLimiterByAccount(formatted string) gin.HandlerFunc {
	store := memory.NewStore()
	instance := limiter.New(store, rateOrDefault(formatted))

	return func(c *gin.Context) {
		key := c.GetString("accountID")
		if key == "" {
			key = "ip:" + c.ClientIP()
		} else {
			key = "acc:" + key
		}

		ctx, err := instance.Get(c, key)
		if err != nil {
			// Fail-open: a broken rate limiter shouldn't take down the API.
			c.Next()
			return
		}
		c.Header("X-RateLimit-Limit", strconv.FormatInt(ctx.Limit, 10))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(ctx.Remaining, 10))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(ctx.Reset, 10))

		if ctx.Reached {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    "RATE_LIMITED",
				"message": "rate limit exceeded for this account — slow down or contact support",
			})
			return
		}
		c.Next()
	}
}
