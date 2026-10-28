package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLogs swaps the default slog logger for a JSON handler writing to a
// buffer, runs fn, then restores the previous logger. Returns the parsed log
// records (one per line).
func captureLogs(t *testing.T, fn func()) []map[string]any {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	fn()

	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &rec))
		records = append(records, rec)
	}
	return records
}

// TestStructuredLogger_ReusesInboundRequestID: when RequestID runs first and
// honors an inbound X-Request-ID, the request log line carries that SAME id as
// request_id and the response echoes it in the X-Request-ID header (US-069).
func TestStructuredLogger_ReusesInboundRequestID(t *testing.T) {
	r := gin.New()
	r.Use(RequestID())
	r.Use(StructuredLogger())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	const inbound = "01HFIXEDREQUESTID0000000000"
	w := httptest.NewRecorder()
	records := captureLogs(t, func() {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("X-Request-ID", inbound)
		r.ServeHTTP(w, req)
	})

	assert.Equal(t, inbound, w.Header().Get("X-Request-ID"), "inbound id echoed in response header")
	require.Len(t, records, 1)
	assert.Equal(t, inbound, records[0]["request_id"], "log line carries the same request_id")
}

// TestStructuredLogger_IncludesAccountAndUserID: the request log line includes
// account_id and user_id when present on the context (US-069).
func TestStructuredLogger_IncludesAccountAndUserID(t *testing.T) {
	r := gin.New()
	r.Use(RequestID())
	r.Use(func(c *gin.Context) {
		c.Set("accountID", "acc_123")
		c.Set("userID", "usr_456")
		c.Next()
	})
	r.Use(StructuredLogger())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	records := captureLogs(t, func() {
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	})

	require.Len(t, records, 1)
	assert.NotEmpty(t, records[0]["request_id"])
	assert.Equal(t, "acc_123", records[0]["account_id"])
	assert.Equal(t, "usr_456", records[0]["user_id"])
}

// TestStructuredLogger_GeneratesIDWhenRequestIDMiddlewareAbsent: even without
// RequestID wired upstream, the logger mints a request_id and sets the header so
// logs stay correlatable (defensive fallback).
func TestStructuredLogger_GeneratesIDWhenRequestIDMiddlewareAbsent(t *testing.T) {
	r := gin.New()
	r.Use(StructuredLogger())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	records := captureLogs(t, func() {
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	})

	require.Len(t, records, 1)
	id, _ := records[0]["request_id"].(string)
	assert.NotEmpty(t, id, "logger mints a request_id when none is set")
	assert.Equal(t, id, w.Header().Get("X-Request-ID"))
}
