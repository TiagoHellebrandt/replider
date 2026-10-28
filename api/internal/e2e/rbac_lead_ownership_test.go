package e2e

import (
	"context"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-F008/F009. The account check answers "is this lead in my company?".
// These tests pin the question it CANNOT answer: "is it mine?" — the IDOR
// one vendedor could otherwise use to read (or worse, edit) a colleague's
// pipeline.

type rbacFixture struct {
	accountID string
	funnelID  string
	vendedorA string
	vendedorB string
	gestor    string
	leadOfA   string
	leadOfB   string
	unowned   string
	privateB  string
	uc        *usecases.LeadUseCase
}

func seedRBACUser(t *testing.T, accountID, role string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO users (id, account_id, name, email, role, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, true, NOW(), NOW())`,
		// ULIDs share their first 10 chars within the same millisecond (that
		// prefix IS the timestamp) — key the email off the random tail.
		id, accountID, role+"-"+id[20:], id[10:]+"@rbac.test", role)
	require.NoError(t, err)
	return id
}

func seedRBACLead(t *testing.T, accountID, funnelID, contactID string, owner *string, visibility string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO leads (id, account_id, funnel_id, contact_id, status, owner_user_id, visibility, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'new', $5, $6, NOW(), NOW())`,
		id, accountID, funnelID, contactID, owner, visibility)
	require.NoError(t, err)
	return id
}

func newRBACFixture(t *testing.T) rbacFixture {
	t.Helper()
	cleanDB(t)
	ctx := context.Background()

	accountID := seedIdentityAccount(t)
	funnelID := ulid.Make().String()
	_, err := testPool.Exec(ctx,
		`INSERT INTO sales_funnels (id, account_id, name, created_at, updated_at)
		 VALUES ($1, $2, 'Funil RBAC', NOW(), NOW())`, funnelID, accountID)
	require.NoError(t, err)

	f := rbacFixture{
		accountID: accountID,
		funnelID:  funnelID,
		vendedorA: seedRBACUser(t, accountID, string(domain.RoleVendedor)),
		vendedorB: seedRBACUser(t, accountID, string(domain.RoleVendedor)),
		gestor:    seedRBACUser(t, accountID, string(domain.RoleGestor)),
	}
	// Distinct contacts: leads are unique per (funnel, contact).
	f.leadOfA = seedRBACLead(t, accountID, funnelID, seedIdentityContact(t, accountID, "+5511900000001"), &f.vendedorA, "team")
	f.leadOfB = seedRBACLead(t, accountID, funnelID, seedIdentityContact(t, accountID, "+5511900000002"), &f.vendedorB, "team")
	f.unowned = seedRBACLead(t, accountID, funnelID, seedIdentityContact(t, accountID, "+5511900000003"), nil, "team")
	f.privateB = seedRBACLead(t, accountID, funnelID, seedIdentityContact(t, accountID, "+5511900000004"), &f.vendedorB, "private")

	f.uc = usecases.NewLeadUseCase(
		postgres.NewLeadRepository(testPool),
		postgres.NewSalesFunnelRepository(testPool),
	)
	// Wired so pipeline reads resolve who is behind each lead: the approval
	// UI shows the recipient before anything is sent, and a contact ULID
	// answers that with nothing.
	f.uc.SetContactRepo(postgres.NewContactRepository(testPool))
	// Owner assignment REFUSES when the user repo is unwired
	// (ErrLeadAssignUnavailable): it is the only check keeping a lead's owner
	// inside its own account, and assigning unchecked is worse than not
	// assigning. Unwired, this fixture asserted the refusal instead of the rule
	// it exists to pin — TestRBAC_OnlyManagersAssignOwners was red for that
	// reason, not for a defect in the rule.
	f.uc.SetUserRepo(postgres.NewUserRepository(testPool, nil))
	return f
}

func (f rbacFixture) actor(userID string, role domain.Role) domain.Actor {
	return domain.Actor{UserID: userID, AccountID: f.accountID, Role: role}
}

func TestRBAC_VendedorCannotReadColleaguesPrivateLead(t *testing.T) {
	f := newRBACFixture(t)
	ctx := context.Background()

	// Own lead: fine.
	own, err := f.uc.GetForActor(ctx, f.actor(f.vendedorA, domain.RoleVendedor), f.leadOfA)
	require.NoError(t, err)
	assert.Equal(t, f.leadOfA, own.ID)

	// Colleague's team-visible lead: the sales floor sees each other's
	// pipeline on purpose.
	shared, err := f.uc.GetForActor(ctx, f.actor(f.vendedorA, domain.RoleVendedor), f.leadOfB)
	require.NoError(t, err)
	assert.Equal(t, f.leadOfB, shared.ID)

	// Colleague's PRIVATE lead: denied.
	_, err = f.uc.GetForActor(ctx, f.actor(f.vendedorA, domain.RoleVendedor), f.privateB)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestRBAC_GestorSeesEverythingIncludingPrivate(t *testing.T) {
	f := newRBACFixture(t)
	ctx := context.Background()

	for _, leadID := range []string{f.leadOfA, f.leadOfB, f.privateB, f.unowned} {
		lead, err := f.uc.GetForActor(ctx, f.actor(f.gestor, domain.RoleGestor), leadID)
		require.NoError(t, err, leadID)
		assert.Equal(t, leadID, lead.ID)
	}
}

// Reading a teammate's lead is allowed (team visibility); CHANGING it is not
// — visibility governs looking, not touching.
func TestRBAC_VendedorCannotMutateColleaguesLead(t *testing.T) {
	f := newRBACFixture(t)
	ctx := context.Background()
	actorA := f.actor(f.vendedorA, domain.RoleVendedor)

	_, err := f.uc.MutableForActor(ctx, actorA, f.leadOfA)
	require.NoError(t, err, "own lead is mutable")

	_, err = f.uc.MutableForActor(ctx, actorA, f.unowned)
	require.NoError(t, err, "an unassigned lead can be picked up")

	_, err = f.uc.MutableForActor(ctx, actorA, f.leadOfB)
	assert.ErrorIs(t, err, domain.ErrForbidden, "a colleague's lead is read-only")
}

// The tenant boundary stays absolute: another account's user is denied even
// with a manager role.
func TestRBAC_CrossAccountIsAlwaysDenied(t *testing.T) {
	f := newRBACFixture(t)
	ctx := context.Background()

	otherAccount := seedIdentityAccount(t)
	stranger := domain.Actor{
		UserID:    seedRBACUser(t, otherAccount, string(domain.RoleGestor)),
		AccountID: otherAccount,
		Role:      domain.RoleGestor,
	}

	_, err := f.uc.GetForActor(ctx, stranger, f.leadOfA)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

// The pipeline list must filter IN SQL. An in-memory filter over an already
// paginated page hides rows that rank past the limit — the exact bug the
// conversation `assigned` filter once had.
func TestRBAC_PipelineListIsScopedForVendedor(t *testing.T) {
	f := newRBACFixture(t)
	ctx := context.Background()

	all, err := f.uc.ListByFunnelForActor(ctx, f.actor(f.gestor, domain.RoleGestor), f.funnelID, "", 50, 0)
	require.NoError(t, err)
	assert.Len(t, all, 4, "a manager sees the whole funnel")

	mine, err := f.uc.ListByFunnelForActor(ctx, f.actor(f.vendedorA, domain.RoleVendedor), f.funnelID, "", 50, 0)
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, l := range mine {
		ids[l.ID] = true
	}
	assert.True(t, ids[f.leadOfA], "own lead")
	assert.True(t, ids[f.unowned], "unassigned leads are up for grabs")
	assert.True(t, ids[f.leadOfB], "team-visible colleague lead")
	assert.False(t, ids[f.privateB], "a colleague's private lead stays hidden")
}

// An attendant is a support role: the sales pipeline is not theirs to browse.
func TestRBAC_AttendantHasNoPipelineAccess(t *testing.T) {
	f := newRBACFixture(t)
	ctx := context.Background()
	attendant := f.actor(seedRBACUser(t, f.accountID, string(domain.RoleAtendenteRole)), domain.RoleAtendenteRole)

	_, err := f.uc.ListByFunnelForActor(ctx, attendant, f.funnelID, "", 50, 0)
	assert.ErrorIs(t, err, domain.ErrForbidden)

	_, err = f.uc.GetForActor(ctx, attendant, f.leadOfA)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

// Reassignment is a manager's call: a vendedor claiming a colleague's lead
// would make the ownership model decorative.
func TestRBAC_OnlyManagersAssignOwners(t *testing.T) {
	f := newRBACFixture(t)
	ctx := context.Background()

	_, err := f.uc.AssignOwner(ctx, f.actor(f.vendedorA, domain.RoleVendedor), f.leadOfB, &f.vendedorA)
	assert.ErrorIs(t, err, domain.ErrForbidden)

	updated, err := f.uc.AssignOwner(ctx, f.actor(f.gestor, domain.RoleGestor), f.unowned, &f.vendedorA)
	require.NoError(t, err)
	require.NotNil(t, updated.OwnerUserID)
	assert.Equal(t, f.vendedorA, *updated.OwnerUserID)

	// Unassigning is the same call with nil.
	cleared, err := f.uc.AssignOwner(ctx, f.actor(f.gestor, domain.RoleGestor), f.unowned, nil)
	require.NoError(t, err)
	assert.Nil(t, cleared.OwnerUserID)
}

// The owner role introduced by the migration must keep passing every gate an
// admin passed, or promoting the account creator silently locks them out.
func TestRBAC_OwnerSatisfiesAdminGates(t *testing.T) {
	assert.True(t, domain.RoleSatisfies(domain.RoleOwner, domain.RoleAdminRole))
	assert.True(t, domain.RoleSatisfies(domain.RoleOwner, domain.RoleAtendenteRole))
	assert.True(t, domain.RoleSatisfies(domain.RoleAdminRole, domain.RoleGestor))
	// ...and the hierarchy does not run backwards.
	assert.False(t, domain.RoleSatisfies(domain.RoleGestor, domain.RoleAdminRole))
	assert.False(t, domain.RoleSatisfies(domain.RoleVendedor, domain.RoleAtendenteRole))
	assert.False(t, domain.RoleSatisfies(domain.RoleAtendenteRole, domain.RoleVendedor))
	// An unknown claim escalates to nothing.
	assert.False(t, domain.RoleSatisfies(domain.Role("root"), domain.RoleAtendenteRole))
}
