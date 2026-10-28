package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
)

// RequestIDKey is the gin context key under which middleware.RequestID stores
// the per-request trace ID. StructuredLogger reads it so every request log line
// carries the SAME request_id that RequestID returns in the X-Request-ID header
// and that response.go echoes in error bodies (US-069).
const RequestIDKey = "requestID"

// StructuredLogger returns Gin middleware that logs every request as structured JSON
// via slog. It reuses the request_id set by middleware.RequestID (which honors an
// inbound X-Request-ID header or mints a ULID) so logs, the response header, and
// error-body metadata all correlate on one ID.
func StructuredLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/ready" {
			c.Next()
			return
		}

		start := time.Now()

		// Reuse the shared request_id from middleware.RequestID, which runs
		// earlier in the chain. Fall back to a freshly minted ULID only if that
		// middleware was not wired (defensive — keeps logs correlatable).
		requestID := c.GetString(RequestIDKey)
		if requestID == "" {
			requestID = ulid.Make().String()
			c.Set(RequestIDKey, requestID)
			c.Header("X-Request-ID", requestID)
		}

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()

		attrs := []slog.Attr{
			slog.String("request_id", requestID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Duration("duration", duration),
			slog.String("client_ip", c.ClientIP()),
		}

		if accountID := c.GetString("accountID"); accountID != "" {
			attrs = append(attrs, slog.String("account_id", accountID))
		}
		if userID := c.GetString("userID"); userID != "" {
			attrs = append(attrs, slog.String("user_id", userID))
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("error", c.Errors.String()))
		}

		args := make([]any, len(attrs))
		for i, a := range attrs {
			args[i] = a
		}

		if status >= 500 {
			slog.Error("request", args...)
		} else if status >= 400 {
			slog.Warn("request", args...)
		} else {
			slog.Info("request", args...)
		}
	}
}
