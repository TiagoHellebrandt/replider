package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestMaxRequestBodySize_RejectsOversizedContentLength: a request declaring a
// body larger than the cap is rejected with 413 before the handler runs.
func TestMaxRequestBodySize_RejectsOversizedContentLength(t *testing.T) {
	r := gin.New()
	r.Use(MaxRequestBodySize(10))
	handlerRan := false
	r.POST("/x", func(c *gin.Context) {
		handlerRan = true
		c.Status(http.StatusOK)
	})

	body := strings.Repeat("a", 100) // 100 bytes > 10 byte cap
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Contains(t, w.Body.String(), "REQUEST_TOO_LARGE")
	assert.False(t, handlerRan, "handler must not run for an oversized body")
}

// TestMaxRequestBodySize_AllowsWithinLimit: a request under the cap reaches the
// handler and its body reads cleanly.
func TestMaxRequestBodySize_AllowsWithinLimit(t *testing.T) {
	r := gin.New()
	r.Use(MaxRequestBodySize(1024))
	var got string
	r.POST("/x", func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		got = string(b)
		c.Status(http.StatusOK)
	})

	body := strings.Repeat("a", 100)
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, body, got)
}

// TestMaxRequestBodySize_CapsStreamedBody: a body with no declared
// Content-Length larger than the cap fails at read time via MaxBytesReader,
// so a lying/absent length can't buffer unbounded memory.
func TestMaxRequestBodySize_CapsStreamedBody(t *testing.T) {
	r := gin.New()
	r.Use(MaxRequestBodySize(10))
	var readErr error
	r.POST("/x", func(c *gin.Context) {
		_, readErr = io.ReadAll(c.Request.Body)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(strings.Repeat("a", 100)))
	req.ContentLength = -1 // unknown length: bypass the fast check, hit MaxBytesReader
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Error(t, readErr, "reading past the cap must fail")
}

// TestMaxRequestBodySize_Disabled: a non-positive cap disables the limit.
func TestMaxRequestBodySize_Disabled(t *testing.T) {
	r := gin.New()
	r.Use(MaxRequestBodySize(0))
	handlerRan := false
	r.POST("/x", func(c *gin.Context) {
		handlerRan = true
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(strings.Repeat("a", 10_000)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, handlerRan)
}

// TestMaxRequestBodySize_WebSocketUpgradeExempt: a WS handshake skips the cap.
func TestMaxRequestBodySize_WebSocketUpgradeExempt(t *testing.T) {
	r := gin.New()
	r.Use(MaxRequestBodySize(10))
	handlerRan := false
	r.GET("/ws", func(c *gin.Context) {
		handlerRan = true
		c.Status(http.StatusSwitchingProtocols)
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", strings.NewReader(strings.Repeat("a", 100)))
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, handlerRan, "WebSocket upgrade must be exempt from the body cap")
}
