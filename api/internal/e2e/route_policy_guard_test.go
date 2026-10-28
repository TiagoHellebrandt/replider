package e2e

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// US-F009 deny-by-default guard.
//
// Authorization in this codebase is per-route middleware, which means a new
// route is unprotected by OMISSION — the failure mode is silent and only
// shows up as a customer reading another customer's data. This test makes
// omission loud: every route the router registers must be classified below,
// and a new one fails CI until someone writes down what protects it.
//
// It is a review gate, not a runtime check: it cannot verify that the
// declared class matches the middleware actually mounted. What it buys is
// that nobody adds a route without stating its protection out loud.

type routeClass string

const (
	// classPublic — deliberately unauthenticated (marketing, webhooks,
	// health, OAuth handshakes). Each one is a decision, not an oversight.
	classPublic routeClass = "public"
	// classAuthed — requires a valid JWT; per-account data only.
	classAuthed routeClass = "authed"
	// classAdmin — RequireRole("admin"): settings, billing, team.
	classAdmin routeClass = "admin"
	// classOwner — super-admin platform routes (/owner/*), gated by the
	// email allowlist + 2FA.
	classOwner routeClass = "owner"
	// classSeller — platform-account sellers only (/seller/*).
	classSeller routeClass = "seller"
	// classMCP — MCP transport; identity resolved from the PAT/OAuth token
	// and re-checked inside every tool.
	classMCP routeClass = "mcp"
)

// routePolicies is the declaration. Key is "METHOD /path" exactly as gin
// reports it.
//
// Prefix rules keep the table from becoming a chore: a route matching a
// prefix below inherits its class. Anything not covered must be listed
// explicitly.
var routePrefixPolicies = []struct {
	prefix string
	class  routeClass
}{
	{"/v1/owner/", classOwner},
	// The OAuth 2.1 handshake is also mounted at the root (the MCP client
	// discovers it via /.well-known), and the legal links page likewise.
	{"/oauth/", classPublic},
	{"/v1/seller/", classSeller},
	{"/v1/webhooks/", classPublic},
	{"/v1/oauth/", classPublic},
	{"/.well-known/", classPublic},
	{"/swagger/", classPublic},
	{"/mcp", classMCP},
}

// explicitRoutes covers everything the prefixes don't. Public entries are the
// ones worth arguing about in review — each is reachable with no token.
var explicitRoutes = map[string]routeClass{
	"GET /health":  classPublic,
	"GET /healthz": classPublic,
	// The dependency breakdown; the operator token is checked inside the
	// handler (METRICS_TOKEN), so the route itself carries no app auth.
	"GET /health/details": classPublic,
	"GET /ready":          classPublic,
	"GET /metrics":        classPublic, // locked down at the proxy, not in app auth

	// Signup + auth handshakes.
	"POST /v1/accounts":                 classPublic,
	"POST /v1/auth/login":               classPublic,
	"POST /v1/auth/2fa/verify-login":    classPublic,
	"POST /v1/auth/forgot-password":     classPublic,
	"POST /v1/auth/reset-password":      classPublic,
	"POST /v1/auth/google":              classPublic,
	"POST /v1/auth/meta":                classPublic,
	"GET /v1/auth/verify-email":         classPublic,
	"POST /v1/auth/resend-verification": classPublic,
	"POST /v1/auth/logout":              classAuthed,

	// Public product surface (marketing site + pricing).
	"GET /v1/config":                  classPublic, // US-F001 signup flag
	"GET /v1/plans":                   classPublic,
	"GET /v1/plans/contact-channels":  classPublic,
	"GET /v1/payments/addon-packs":    classPublic,
	"POST /v1/payments/webhook":       classPublic, // Stripe HMAC
	"GET /v1/templates/prompts":       classPublic,
	"POST /v1/newsletter/subscribe":   classPublic,
	"POST /v1/sales/qualify":          classPublic,
	"POST /v1/roi-leads":              classPublic,
	"GET /v1/coupons/:code":           classPublic,
	"GET /v1/legal":                   classPublic,
	"GET /legal":                      classPublic,
	"GET /v1/telegram/file":           classPublic,
	"GET /v1/agent-templates":         classPublic,
	"POST /v1/whatsapp/flows/data":    classPublic, // Meta-signed
	"POST /v1/whatsapp/flows/refresh": classPublic,
}

func routeKey(method, path string) string { return method + " " + path }

func classify(method, path string) (routeClass, bool) {
	if c, ok := explicitRoutes[routeKey(method, path)]; ok {
		return c, true
	}
	for _, p := range routePrefixPolicies {
		if strings.HasPrefix(path, p.prefix) {
			return p.class, true
		}
	}
	// Everything else under /v1 is expected to be an authenticated,
	// account-scoped route. That default is safe ONLY because the
	// undeclared-public case is what this test hunts: a genuinely public
	// route that isn't in the table above shows up as a false "authed"
	// classification, which is exactly what review must catch.
	if strings.HasPrefix(path, "/v1/") {
		return classAuthed, true
	}
	return "", false
}

func TestRoutePolicy_EveryRouteIsClassified(t *testing.T) {
	var unclassified []string
	for _, r := range testRouter.Routes() {
		if _, ok := classify(r.Method, r.Path); !ok {
			unclassified = append(unclassified, routeKey(r.Method, r.Path))
		}
	}
	sort.Strings(unclassified)
	assert.Empty(t, unclassified,
		"routes outside /v1 must be classified in route_policy_guard_test.go: %v", unclassified)
}

// The public surface is the attack surface. Pinning its SIZE makes growing
// it a deliberate, reviewed diff: a route that becomes reachable with no
// token — whether by a new registration or by an auth middleware quietly
// disappearing — fails here until someone updates the number and says why.
//
// Counted over the LIVE router, not over the declaration table: the table
// documents intent, the router is the truth.
func TestRoutePolicy_PublicSurfaceIsPinned(t *testing.T) {
	var public []string
	for _, r := range testRouter.Routes() {
		if class, ok := classify(r.Method, r.Path); ok && class == classPublic {
			public = append(public, routeKey(r.Method, r.Path))
		}
	}
	sort.Strings(public)

	assert.Len(t, public, publicRouteCount,
		"the anonymous-reachable surface changed:\n%s\n"+
			"update publicRouteCount only after reviewing WHY each new route may be anonymous",
		strings.Join(public, "\n"))
}

// publicRouteCount is the number of routes reachable with no token in the
// e2e harness. NOTE: the harness mounts the subset of handlers the tests
// need, so this is a floor on the production surface, not a full census —
// its job is to make CHANGE visible, not to enumerate production.
//
// 37 since the buying-signal ingestion webhook (§2 D2): machine-to-machine,
// no session, authenticated by an HMAC over the body. It is anonymous in the
// JWT sense on purpose — the caller is a Zapier/Make/n8n job, not a user —
// and it refuses outright when no signing secret is configured, so an
// unsigned write is never accepted.
//
// 38 since the signature provider callback `POST /v1/webhooks/signature/:token`
// (§6.11 G7). Reviewed rather than rubber-stamped: the token in the PATH is the
// credential — `SignatureUseCase.HandleCallback` resolves the account through
// `settings.ByWebhookToken` and an unknown token gets the same "not found" as a
// wrong one, so it leaks nothing about which accounts exist. Same class as
// `POST /v1/webhooks/:channel_token`. The body read is bounded at 1 MiB and
// every failure answers 200 "ignored", so a stranger learns nothing from the
// response either.
//
// This guard was red for both merges in between and nobody saw it: CI runs
// `go test $(go list ./... | grep -v /e2e)`, so the whole package is outside the
// gate, and locally the suite exceeds `go test`'s default 10m timeout — the
// panic aborted the run before this test was reached. Run the package with
// `-timeout 25m` when touching routes.
// 39 since 2026-09-01: GET /health/details joined the anonymous surface —
// reviewed: the handler refuses every caller without METRICS_TOKEN (and
// refuses everyone when the token is unset), so the route carries no app
// auth but the page is not anonymous.
const publicRouteCount = 39
