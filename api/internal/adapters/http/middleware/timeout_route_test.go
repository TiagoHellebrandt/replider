package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// Reading a manufacturer's brochure is one model call over a whole document
// and the 30s default cut the bigger catalogues off mid-read, reporting "a
// leitura automática falhou" — which sounded like a broken file.
//
// The override lives in the middleware because a per-route Timeout applied
// later would derive from the already-30s context and could only shorten it.

func deadlineFor(t *testing.T, route, path string) time.Duration {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Timeout(30 * time.Second))
	var budget time.Duration
	r.POST(route, func(c *gin.Context) {
		dl, ok := c.Request.Context().Deadline()
		if !ok {
			t.Fatal("no deadline on the request context")
		}
		budget = time.Until(dl).Round(time.Second)
		c.Status(http.StatusOK)
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, path, nil))
	return budget
}

func TestTimeout_RouteOverrideAppliesToTheBrochureRead(t *testing.T) {
	if got := deadlineFor(t, "/v1/products/import/pdf", "/v1/products/import/pdf"); got != 100*time.Second {
		t.Fatalf("budget = %s, want 100s", got)
	}
}

func TestTimeout_OtherRoutesKeepTheDefault(t *testing.T) {
	if got := deadlineFor(t, "/v1/plans", "/v1/plans"); got != 30*time.Second {
		t.Fatalf("budget = %s, want 30s", got)
	}
}

// The override must not leak: d is captured by the middleware's closure, so
// assigning to it would hand 100s to every later request on the process.
func TestTimeout_OverrideDoesNotLeakToTheNextRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Timeout(30 * time.Second))
	var budget time.Duration
	record := func(c *gin.Context) {
		dl, _ := c.Request.Context().Deadline()
		budget = time.Until(dl).Round(time.Second)
		c.Status(http.StatusOK)
	}
	r.POST("/v1/products/import/pdf", record)
	r.GET("/v1/plans", record)

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/products/import/pdf", nil))
	if budget != 100*time.Second {
		t.Fatalf("first request budget = %s, want 100s", budget)
	}
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/plans", nil))
	if budget != 30*time.Second {
		t.Fatalf("the override leaked: second request got %s, want 30s", budget)
	}
}

// The routing PATTERN, not the raw path — so no caller can widen its own
// budget by crafting a URL that merely looks like the privileged one.
func TestTimeout_OverrideKeysOnTheRoutePattern(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Timeout(30 * time.Second))
	var budget time.Duration
	r.POST("/v1/products/:id", func(c *gin.Context) {
		dl, _ := c.Request.Context().Deadline()
		budget = time.Until(dl).Round(time.Second)
		c.Status(http.StatusOK)
	})
	// Matches the wildcard route, so the handler DOES run — and FullPath is
	// "/v1/products/:id", which is not a key in routeTimeouts.
	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/v1/products/import", nil))
	if budget != 30*time.Second {
		t.Fatalf("a path that merely resembles the privileged route bought %s, want 30s", budget)
	}
}
