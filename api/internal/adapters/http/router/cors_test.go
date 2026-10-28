package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// newCORSEngine builds a minimal gin engine wired with the SAME CORS gate the
// real router applies (originAllowed over a parsed allow-list), so the test
// exercises the production matcher rather than a stand-in.
func newCORSEngine(raw string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	allowedOrigins := buildAllowedOrigins(raw)
	r := gin.New()
	r.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			return originAllowed(allowedOrigins, origin)
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	return r
}

// corsOrigin issues a CORS preflight (OPTIONS) for origin against eng and
// returns the Access-Control-Allow-Origin header the middleware echoes back.
// gin-contrib/cors only sets that header when AllowOriginFunc approves the
// origin, so an empty value means the origin was blocked.
func corsOrigin(eng *gin.Engine, origin string) string {
	req := httptest.NewRequest(http.MethodOptions, "/ping", nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	eng.ServeHTTP(rec, req)
	return rec.Header().Get("Access-Control-Allow-Origin")
}

// TestCORS_BlocksDisallowedOrigin is the AC#2 guard for US-080: with an
// explicit allow-list configured, a request from an origin NOT on the list
// must be refused (no Access-Control-Allow-Origin reflected back), while a
// listed origin is allowed.
func TestCORS_BlocksDisallowedOrigin(t *testing.T) {
	eng := newCORSEngine("https://app.replider.com,https://replider.com")

	if got := corsOrigin(eng, "https://evil.example.com"); got != "" {
		t.Fatalf("disallowed origin should be blocked, but got Access-Control-Allow-Origin=%q", got)
	}

	if got := corsOrigin(eng, "https://app.replider.com"); got != "https://app.replider.com" {
		t.Fatalf("allowed origin should be permitted, got Access-Control-Allow-Origin=%q", got)
	}

	// Case-insensitive host match still allows a configured origin.
	if got := corsOrigin(eng, "https://APP.replider.com"); got == "" {
		t.Fatal("case-insensitive match of a configured origin should be allowed")
	}
}

// TestOriginAllowed is the unit-level table for the matcher backing the CORS
// gate, including the dev fallback (empty allow-list allows everything).
func TestOriginAllowed(t *testing.T) {
	allow := []string{"https://app.replider.com", "https://replider.com"}
	cases := []struct {
		name    string
		allowed []string
		origin  string
		want    bool
	}{
		{"exact match", allow, "https://app.replider.com", true},
		{"second entry", allow, "https://replider.com", true},
		{"case-insensitive", allow, "https://App.Replider.com", true},
		{"disallowed origin", allow, "https://evil.example.com", false},
		{"empty origin", allow, "", false},
		{"substring is not a match", allow, "https://app.replider.com.evil.com", false},
		{"empty allow-list = dev fallback", nil, "https://anything.test", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := originAllowed(tc.allowed, tc.origin); got != tc.want {
				t.Fatalf("originAllowed(%v, %q) = %v, want %v", tc.allowed, tc.origin, got, tc.want)
			}
		})
	}
}
