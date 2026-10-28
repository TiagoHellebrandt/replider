package handlers

import (
	"testing"

	"github.com/replider/api/internal/core/domain"
)

// The dashboard socket's gate must match the dashboard's own
// (middleware.RequireRole("admin", "attendant")). It was a literal
// `role == "admin" || role == "attendant"` comparison until US-F008
// introduced owner/gestor/vendedor — which left an OWNER, the account's
// highest-privileged user, as the only one with no realtime, reconnecting
// every 3s forever.
func TestWebSocketRoleGate_MatchesTheDashboardGate(t *testing.T) {
	allowed := func(raw string) bool {
		role := domain.NormalizeRole(raw)
		return domain.RoleSatisfies(role, domain.RoleAdminRole) ||
			domain.RoleSatisfies(role, domain.RoleAtendenteRole)
	}

	for _, role := range []string{"owner", "admin", "gestor", "attendant"} {
		if !allowed(role) {
			t.Errorf("%q may open the inbox, so it must receive the inbox's events", role)
		}
	}

	// A vendedor works the pipeline, not the inbox — deliberately excluded
	// here exactly as they are from the dashboard routes.
	if allowed("vendedor") {
		t.Error("a vendedor must not get the dashboard socket")
	}
	// An unknown role degrades to attendant (NormalizeRole), which is the
	// least-privileged role that still works the inbox — the same
	// behaviour every other RBAC check has.
	if !allowed("something-else") {
		t.Error("an unknown role must degrade the same way it does elsewhere")
	}
}
