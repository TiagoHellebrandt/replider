package router

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/http/middleware"
)

// A per-route timeout is keyed by the gin route PATTERN. Get the string wrong
// — a missing /v1, a renamed segment — and nothing breaks loudly: the route
// quietly keeps the default budget, and the only symptom is the timeout the
// override existed to prevent. Reading "/v1/products/import/pdf" in two files
// and believing they match is exactly the kind of agreement that rots.
//
// So this asserts the keys against the routes the router really registers.

func TestTimeoutRouteKeysMatchRegisteredRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/v1")
	pass := func(c *gin.Context) { c.Next() }

	// Every group that owns a key in routeTimeouts has to be registered here.
	// A new key in another group fails this test with the message below, which
	// is the point: it asks for the harness to grow WITH the map.
	setupProductImportRoutes(v1, &handlers.ProductImportHandler{}, pass)

	registered := make(map[string]bool)
	for _, ri := range r.Routes() {
		registered[ri.Path] = true
	}

	for _, key := range middleware.TimeoutRouteKeys() {
		if !registered[key] {
			t.Fatalf("routeTimeouts has %q, which no route registered here matches.\n"+
				"Either the pattern is wrong (the override does nothing and the route "+
				"silently keeps the default budget), or the route lives in a group this "+
				"test does not set up yet — add it above.\nRegistered: %s",
				key, strings.Join(keysOf(registered), ", "))
		}
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
