package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/handlers"
)

// TestWebhookRoutesDoNotCollideWithInbound is the smoke test that
// would have caught the production-launch panic on Apr 25 2026:
//
//	panic: ':channel_token' in new path '/v1/webhooks/:channel_token'
//	conflicts with existing wildcard ':id' in existing prefix
//	'/v1/webhooks/:id'
//
// gin's router tree treats two routes with the same path shape but
// different param names as a hard collision. The legacy public
// inbound route lives at `/webhooks/:channel_token` (Telegram + webchat
// post here), and the admin outbound webhook CRUD landed under
// `/webhooks/:id`. Both registered against the same v1 group.
//
// The fix moved admin CRUD to `/account-webhooks` so the prefixes no
// longer overlap. This test re-creates the same registration order
// against a real *gin.Engine; if anyone re-introduces an `:id`-style
// admin route under `/webhooks` (or any other inbound prefix), the
// engine panics on registration and the test fails BEFORE deploy.
func TestWebhookRoutesDoNotCollideWithInbound(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("router setup panicked — likely a route-prefix collision: %v", r)
		}
	}()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	v1 := engine.Group("/v1")

	// Stand-in for the public Telegram/webchat inbound route — same
	// shape gin sees in production.
	v1.POST("/webhooks/:channel_token", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Real admin webhook routes under their new prefix. If someone
	// flips the prefix back to `/webhooks` (or makes a sibling collide),
	// the addRoute call panics here.
	authMw := func(c *gin.Context) { c.Next() }
	setupWebhookRoutes(v1, handlers.NewAccountWebhookHandler(nil), authMw)
}

// TestWebhookRoutesNilHandlerNoOps confirms the setup function bails
// early when no handler is wired. Keeps test/dev wiring without the
// outbound dispatcher (and the fakes that used to crash because
// HandleList tried to deref a nil repo) functional.
func TestWebhookRoutesNilHandlerNoOps(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil handler should be a no-op; got panic: %v", r)
		}
	}()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	v1 := engine.Group("/v1")
	authMw := func(c *gin.Context) { c.Next() }
	setupWebhookRoutes(v1, nil, authMw)
}
