package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { gin.SetMode(gin.TestMode) }

// TestPrometheusMetricsExposed drives a request through the metrics
// middleware and then scrapes the default registry via promhttp to prove the
// http_requests_total counter and http_request_duration_seconds histogram
// (US-067) are both registered and populated with the route PATTERN label.
func TestPrometheusMetricsExposed(t *testing.T) {
	r := gin.New()
	r.Use(PrometheusMetrics())
	r.GET("/ping/:id", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Drive a request so the counter/histogram move.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping/123", nil))
	require.Equal(t, http.StatusOK, w.Code)

	// Scrape the metrics endpoint.
	scrape := httptest.NewRecorder()
	r.ServeHTTP(scrape, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, scrape.Code)

	body := scrape.Body.String()
	assert.Contains(t, body, "http_requests_total")
	assert.Contains(t, body, "http_request_duration_seconds")
	// Path is labelled by the route pattern, not the concrete id, so
	// cardinality stays bounded.
	assert.Contains(t, body, `path="/ping/:id"`)
	assert.NotContains(t, body, `path="/ping/123"`)
	// Sanity: the request we drove was counted.
	assert.True(t, strings.Contains(body, `http_requests_total{method="GET",path="/ping/:id",status="200"}`),
		"expected the driven request to be counted, got:\n%s", body)
}
