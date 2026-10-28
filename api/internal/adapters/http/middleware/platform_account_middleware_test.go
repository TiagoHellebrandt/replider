package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func performPlatformRequest(platformID, accountID string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/seller/ping", func(c *gin.Context) {
		if accountID != "" {
			c.Set("accountID", accountID)
		}
	}, RequirePlatformAccount(platformID), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/seller/ping", nil)
	r.ServeHTTP(w, req)
	return w
}

func TestRequirePlatformAccount_AllowsPlatformAccount(t *testing.T) {
	w := performPlatformRequest("acc-platform", "acc-platform")
	assert.Equal(t, http.StatusOK, w.Code)
}

// Outside the platform account the route must not exist as far as the
// caller can tell — 404, never 403.
func TestRequirePlatformAccount_404ForOtherAccounts(t *testing.T) {
	w := performPlatformRequest("acc-platform", "acc-customer")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "NOT_FOUND")
}

// PLATFORM_ACCOUNT_ID unset ⇒ fail closed: even the would-be platform
// account gets a 404.
func TestRequirePlatformAccount_404WhenUnconfigured(t *testing.T) {
	w := performPlatformRequest("", "acc-platform")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Missing auth context (middleware misordering) must also 404, not 200.
func TestRequirePlatformAccount_404WithoutAccountID(t *testing.T) {
	w := performPlatformRequest("acc-platform", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}
