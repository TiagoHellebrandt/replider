package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	limiter "github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// defaultRateFormatted is the fallback applied when a configured rate
// string fails to parse. A typo in an optional env var must not crash
// the API at boot — degrade to a safe default and log loudly instead.
const defaultRateFormatted = "120-M"

// rateOrDefault parses a rate string like "10-M" (10/minute), falling
// back to defaultRateFormatted with an error log on a malformed value.
func rateOrDefault(formatted string) limiter.Rate {
	rate, err := limiter.NewRateFromFormatted(formatted)
	if err != nil {
		slog.Error("invalid rate limit format — falling back to default",
			"value", formatted, "fallback", defaultRateFormatted)
		rate, _ = limiter.NewRateFromFormatted(defaultRateFormatted)
	}
	return rate
}

// RateLimiter returns a Gin middleware that enforces per-IP rate limiting.
// `formatted` is a rate string like "10-M" (10 requests per minute) or "100-H" (100 per hour).
func RateLimiter(formatted string) gin.HandlerFunc {
	store := memory.NewStore()
	instance := limiter.New(store, rateOrDefault(formatted))
	return mgin.NewMiddleware(instance)
}
