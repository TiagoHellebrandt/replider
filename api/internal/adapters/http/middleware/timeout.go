package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Timeout returns middleware that attaches a hard deadline to every
// request's context (US-034). Downstream ctx-aware work — DB queries,
// the AI provider call, outbound HTTP — observes the deadline and aborts
// instead of holding a goroutine and a connection open indefinitely.
// Combined with the server's ReadHeaderTimeout/ReadTimeout, this blocks
// slowloris-style connection-holding: a client that opens a request and
// then stalls can occupy a worker for at most d.
//
// WebSocket upgrades are exempt: those connections are long-lived by
// design (the dashboard live feed) and manage their own read/write
// deadlines in the ws handler's read/write pumps. Capping them at d would
// drop every dashboard socket after d.
// A handful of routes do work that legitimately outlives the default budget.
// They are named HERE, at the one place the deadline is set, because the
// alternative does not work: this middleware runs first, so a later
// per-route Timeout would derive from an already-expired-in-30s context and
// could only ever shorten it. Gin has matched the route by the time
// middleware runs, so c.FullPath() is the routing pattern, not the raw path
// — no per-request string building, no way for a caller to widen its own
// budget by crafting a URL.
//
// Reading a manufacturer's brochure is one model call over a whole document;
// the 30s default cut off the bigger catalogues mid-read and reported "a
// leitura automática falhou", which sounded like a broken file.
//
// Keep every value below the server's WriteTimeout — a handler that outlives
// it has its connection torn down with no response at all, which is the same
// timeout with none of the explanation.
var routeTimeouts = map[string]time.Duration{
	"/v1/products/import/pdf": 100 * time.Second,
}

// TimeoutRouteKeys lists the routes carrying a custom budget.
//
// Exported for ONE reason: a key that does not match a registered route
// pattern is a fix that silently does nothing — the route keeps the default
// and the only symptom is the timeout it was supposed to prevent. The router
// package asserts these against the routes it actually registers.
func TimeoutRouteKeys() []string {
	keys := make([]string, 0, len(routeTimeouts))
	for k := range routeTimeouts {
		keys = append(keys, k)
	}
	return keys
}

func Timeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isWebSocketUpgrade(c) {
			c.Next()
			return
		}
		// Per REQUEST, never assigning to d: d is captured by this closure, so
		// writing to it would hand the override to every later request on the
		// process — one brochure upload silently raising the deadline for the
		// whole API.
		budget := d
		if override, ok := routeTimeouts[c.FullPath()]; ok {
			budget = override
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), budget)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// isWebSocketUpgrade reports whether the request is a WebSocket handshake,
// which must skip the request deadline.
func isWebSocketUpgrade(c *gin.Context) bool {
	return strings.EqualFold(c.GetHeader("Upgrade"), "websocket") ||
		strings.Contains(strings.ToLower(c.GetHeader("Connection")), "upgrade")
}
