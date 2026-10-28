package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() { gin.SetMode(gin.TestMode) }

// TestTimeout_SetsDeadline: a normal request gets a context deadline
// roughly d out, so downstream ctx-aware work is bounded.
func TestTimeout_SetsDeadline(t *testing.T) {
	r := gin.New()
	r.Use(Timeout(30 * time.Second))
	var hadDeadline bool
	var remaining time.Duration
	r.GET("/x", func(c *gin.Context) {
		dl, ok := c.Request.Context().Deadline()
		hadDeadline = ok
		if ok {
			remaining = time.Until(dl)
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

	assert.True(t, hadDeadline, "request context must carry a deadline")
	assert.Greater(t, remaining, 25*time.Second, "deadline should be ~30s out")
	assert.LessOrEqual(t, remaining, 30*time.Second)
}

// TestTimeout_CancelsSlowHandler: a handler that blocks until ctx.Done()
// is released by the deadline rather than hanging — this is the
// slowloris/hung-connection defense in miniature. Uses a tiny duration so
// the test is fast.
func TestTimeout_CancelsSlowHandler(t *testing.T) {
	r := gin.New()
	r.Use(Timeout(50 * time.Millisecond))

	released := make(chan struct{})
	r.GET("/slow", func(c *gin.Context) {
		<-c.Request.Context().Done() // would hang forever without the deadline
		close(released)
		c.Status(http.StatusGatewayTimeout)
	})

	done := make(chan struct{})
	go func() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/slow", nil))
		close(done)
	}()

	select {
	case <-done:
		// handler returned because ctx was cancelled by the deadline
	case <-time.After(2 * time.Second):
		t.Fatal("handler was not released by the request deadline")
	}
	select {
	case <-released:
	default:
		t.Fatal("ctx.Done() never fired")
	}
}

// TestTimeout_WebSocketUpgradeExempt: a WS handshake must NOT get a
// deadline, otherwise the long-lived dashboard socket would be torn down.
func TestTimeout_WebSocketUpgradeExempt(t *testing.T) {
	r := gin.New()
	r.Use(Timeout(30 * time.Second))
	var hadDeadline bool
	r.GET("/ws", func(c *gin.Context) {
		_, hadDeadline = c.Request.Context().Deadline()
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.False(t, hadDeadline, "WebSocket upgrade must be exempt from the request deadline")
}
