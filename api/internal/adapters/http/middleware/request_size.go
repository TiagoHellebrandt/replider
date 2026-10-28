package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// MaxRequestBodySize returns middleware that blocks oversized request bodies
// (US-192). It guards in two layers:
//
//  1. A fast Content-Length check: a client that declares a body larger than
//     maxBytes is rejected up front with 413 before a single byte is read.
//  2. http.MaxBytesReader wraps the body so a chunked/streamed request with no
//     (or a lying) Content-Length is still capped at read time — a downstream
//     read past the limit fails instead of buffering unbounded memory.
//
// A non-positive maxBytes disables the limit (pass-through). WebSocket upgrades
// are exempt: their long-lived framed connections are not request bodies and
// manage their own per-message read deadlines.
func MaxRequestBodySize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 || isWebSocketUpgrade(c) {
			c.Next()
			return
		}

		if c.Request.ContentLength > maxBytes {
			rejectTooLarge(c)
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

func rejectTooLarge(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
		"code":    "REQUEST_TOO_LARGE",
		"message": "request body exceeds the maximum allowed size",
	})
}
