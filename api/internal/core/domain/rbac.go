package domain

// RBAC model for the SDR pivot (US-F008/F009).
//
// The pre-pivot product had two per-account roles (admin, attendant) plus a
// platform-level super_admin resolved from an env allowlist. Outbound sales
// needs finer granularity: a salesperson must see THEIR pipeline and nobody
// else's, a manager must see the whole team's, and only the owner touches
// billing and commission rules.
//
// Two orthogonal questions, deliberately kept apart:
//
//	role  — WHAT KIND of thing may this user do?      (this file)
//	scope — WHICH ROWS of that thing may they touch?  (ResourceScope below)
//
// Conflating them is how "the manager can't see their own team's leads"
// bugs get written.

// Role is a per-account role. super_admin is NOT here: it is platform-level,
// lives outside the account model, and is resolved by its own middleware.
type Role string

const (
	// RoleOwner owns the account: billing, terms, deleting the account,
	// managing admins, commission rules.
	RoleOwner Role = "owner"
	// RoleAdminRole is everything operational — funnels, team, strategies,
	// contracts, integrations. (Named to avoid colliding with the legacy
	// RoleAdmin string constant kept for compatibility.)
	RoleAdminRole Role = "admin"
	// RoleGestor sees and approves for the WHOLE team, but never touches
	// billing, integrations or commission rules.
	RoleGestor Role = "gestor"
	// RoleVendedor sees only what is theirs: assigned leads, own portfolio,
	// own commission statement.
	RoleVendedor Role = "vendedor"
	// RoleAtendenteRole is the legacy inbox role (customer support), kept
	// unchanged so existing accounts behave exactly as before.
	RoleAtendenteRole Role = "attendant"
)

// satisfies encodes which roles a given role can stand in for. It is an
// explicit table rather than a numeric rank because the hierarchy is NOT a
// straight line: a vendedor outranks nobody, and must NOT inherit the
// attendant's full-inbox access just because they are "senior" to it.
var satisfies = map[Role]map[Role]bool{
	RoleOwner: {
		RoleOwner: true, RoleAdminRole: true, RoleGestor: true,
		RoleVendedor: true, RoleAtendenteRole: true,
	},
	RoleAdminRole: {
		RoleAdminRole: true, RoleGestor: true,
		RoleVendedor: true, RoleAtendenteRole: true,
	},
	// A gestor runs the sales floor and can work the inbox, but is not an
	// admin: no billing, no integrations, no commission rules.
	RoleGestor: {
		RoleGestor: true, RoleVendedor: true, RoleAtendenteRole: true,
	},
	// A vendedor is deliberately NOT an attendant: opening every
	// conversation in the account is not part of the job.
	RoleVendedor:      {RoleVendedor: true},
	RoleAtendenteRole: {RoleAtendenteRole: true},
}

// RoleSatisfies reports whether actor can act in the place of required.
// Unknown roles satisfy nothing — deny by default.
func RoleSatisfies(actor, required Role) bool {
	allowed, ok := satisfies[actor]
	if !ok {
		return false
	}
	return allowed[required]
}

// ValidRole reports whether r is a known per-account role.
func ValidRole(r Role) bool {
	_, ok := satisfies[r]
	return ok
}

// NormalizeRole maps a stored/JWT value onto a known role. An empty or
// unrecognized value degrades to attendant — the least-privileged role —
// so a corrupt claim can never escalate.
func NormalizeRole(raw string) Role {
	r := Role(raw)
	if ValidRole(r) {
		return r
	}
	return RoleAtendenteRole
}

// ── Resource-level authorization (US-F009) ─────────────────────────────────

// ResourceVisibility controls who inside the account can see a row that has
// an owner. `team` is the default: sales works better when the floor can see
// each other's pipeline. `private` restricts to the owner (plus gestor and
// above, who are accountable for the team's numbers).
type ResourceVisibility string

const (
	VisibilityTeam    ResourceVisibility = "team"
	VisibilityPrivate ResourceVisibility = "private"
)

func (v ResourceVisibility) Valid() bool {
	return v == VisibilityTeam || v == VisibilityPrivate
}

// Actor is the authenticated caller, as far as authorization is concerned.
type Actor struct {
	UserID    string
	AccountID string
	Role      Role
}

// ResourceOwnership is the authorization-relevant slice of any owned row
// (a lead today; portfolio clients, strategies and connections next).
type ResourceOwnership struct {
	AccountID   string
	OwnerUserID string // empty = unassigned (nobody's yet)
	Visibility  ResourceVisibility
}

// ErrForbidden is the single denial returned by the resource check. It is a
// 403 rather than a 404 because the caller IS inside the right account —
// hiding existence would only confuse a legitimate teammate.
var ErrForbidden = &AppError{Code: "FORBIDDEN", Message: "insufficient permissions for this resource"}

// CanAccessResource is the single decision point for "may this actor touch
// this row?", shared by HTTP handlers and MCP tools so a tool can never be
// more permissive than the endpoint it mirrors.
//
// Rules, in order:
//  1. Cross-account is always denied (the tenant boundary is absolute).
//  2. gestor and above see the whole team, private rows included — they
//     answer for the team's numbers.
//  3. A vendedor sees their own rows, and their teammates' non-private ones.
//  4. Anyone else (attendant) has no business in owned sales rows.
func CanAccessResource(actor Actor, res ResourceOwnership) bool {
	if actor.AccountID == "" || res.AccountID == "" || actor.AccountID != res.AccountID {
		return false
	}
	if RoleSatisfies(actor.Role, RoleGestor) {
		return true
	}
	if actor.Role != RoleVendedor {
		return false
	}
	if res.OwnerUserID == "" || res.OwnerUserID == actor.UserID {
		return true
	}
	return res.Visibility != VisibilityPrivate
}

// CanMutateResource is stricter than reading: a vendedor may only change
// rows that are theirs (or unassigned), never a teammate's — visibility
// governs looking, not touching.
func CanMutateResource(actor Actor, res ResourceOwnership) bool {
	if actor.AccountID == "" || res.AccountID == "" || actor.AccountID != res.AccountID {
		return false
	}
	if RoleSatisfies(actor.Role, RoleGestor) {
		return true
	}
	if actor.Role != RoleVendedor {
		return false
	}
	return res.OwnerUserID == "" || res.OwnerUserID == actor.UserID
}
