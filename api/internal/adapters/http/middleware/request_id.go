package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
)

// RequestID adds a unique X-Request-ID header to every request for tracing.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = ulid.Make().String()
		}
		c.Set(RequestIDKey, requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}
