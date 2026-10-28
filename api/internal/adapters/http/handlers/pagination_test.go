package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestPaginationDefaultsAndCap verifies the shared list-endpoint pagination
// helper applies a default of 50 and hard-caps limit at 200 (US-061).
func TestPaginationDefaultsAndCap(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantLimit  int
		wantOffset int
	}{
		{"no params → default 50/0", "", 50, 0},
		{"explicit limit honored", "limit=25", 25, 0},
		{"limit at cap", "limit=200", 200, 0},
		{"limit above cap clamped to 200", "limit=999", 200, 0},
		{"huge limit clamped to 200", "limit=1000000", 200, 0},
		{"zero limit floored to 1", "limit=0", 1, 0},
		{"negative limit floored to 1", "limit=-5", 1, 0},
		{"garbage limit → floored to 1", "limit=abc", 1, 0},
		{"offset honored", "limit=10&offset=30", 10, 30},
		{"negative offset floored to 0", "offset=-9", 50, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/?"+tc.query, nil)

			limit, offset := pagination(c)
			assert.Equal(t, tc.wantLimit, limit, "limit")
			assert.Equal(t, tc.wantOffset, offset, "offset")
			assert.LessOrEqual(t, limit, 200, "limit must never exceed 200")
			assert.GreaterOrEqual(t, limit, 1, "limit must never be below 1")
		})
	}
}
