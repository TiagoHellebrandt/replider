package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// The AbacatePay secret is read from the X-Webhook-Secret header first and
// only then from the legacy `?webhookSecret=` query (which access logs
// would otherwise capture).
func TestAbacateWebhookSecret_HeaderBeatsQuery(t *testing.T) {
	ctx := func(url, header string) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, url, nil)
		if header != "" {
			c.Request.Header.Set("X-Webhook-Secret", header)
		}
		return c
	}
	assert.Equal(t, "from-header", abacateWebhookSecret(ctx("/v1/payments/abacatepay/webhook?webhookSecret=from-query", "from-header")))
	assert.Equal(t, "from-query", abacateWebhookSecret(ctx("/v1/payments/abacatepay/webhook?webhookSecret=from-query", "")))
	assert.Equal(t, "", abacateWebhookSecret(ctx("/v1/payments/abacatepay/webhook", "  ")))
}
