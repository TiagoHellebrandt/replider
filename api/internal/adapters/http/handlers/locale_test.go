package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAcceptLocale(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"empty header → default pt", "", "pt"},
		{"plain en", "en", "en"},
		{"plain pt", "pt", "pt"},
		{"region en-US strips to en", "en-US", "en"},
		{"region pt-BR strips to pt", "pt-BR", "pt"},
		{"underscore variant pt_br", "pt_BR", "pt"},
		{"first tag wins", "en, pt;q=0.5", "en"},
		{"unsupported locale falls back", "fr-FR", "pt"},
		{"mixed casing", "EN-us", "en"},
		{"whitespace tolerated", "  en  ;q=1.0", "en"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/", nil)
			if tc.header != "" {
				c.Request.Header.Set("Accept-Language", tc.header)
			}
			assert.Equal(t, tc.want, AcceptLocale(c))
		})
	}
}
