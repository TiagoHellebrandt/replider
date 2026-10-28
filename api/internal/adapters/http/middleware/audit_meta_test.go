package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

// TestAuditMeta_StampsOriginOnContext: the middleware puts the client IP +
// User-Agent onto the request context where the audit writer reads them
// back (US-073).
func TestAuditMeta_StampsOriginOnContext(t *testing.T) {
	r := gin.New()
	r.Use(AuditMeta())
	var meta domain.AuditMeta
	r.GET("/x", func(c *gin.Context) {
		meta = domain.GetAuditMeta(c.Request.Context())
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("User-Agent", "audit-test-agent/1.0")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "audit-test-agent/1.0", meta.UserAgent)
	assert.NotEmpty(t, meta.IP, "client IP should be captured")
}
